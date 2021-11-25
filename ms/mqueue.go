package ms

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// =============================================================================

// MsgEnvelope holds the Message itself and the time when it was added
// to the queue and the Sender id who sent the Message into the queue.
type MsgEnvelope struct {
	Message

	Registered time.Time
	sender     uuid.UUID
	queue      *mQueue
}

// msgRegRequest is used for message registration on server.
type msgRegRequest struct {
	sender uuid.UUID
	msg    Message
}

// Message Queue
type mQueue struct {
	id       uuid.UUID
	name     string
	messages []*MsgEnvelope

	// lastReaded holds the last readed message id for the
	// particular reader
	lastReaded map[uuid.UUID]int

	log *zap.SugaredLogger

	regCh  chan msgRegRequest
	stopCh chan struct{}
}

// ID returns a queue's id
func (q mQueue) ID() uuid.UUID {
	return q.id
}

// Name returns queue's name
func (q mQueue) Name() string {
	return q.name
}

// regLoop gets message registration request from user q.PutMessages
func (q *mQueue) regLoop() {
	for {
		select {
		case <-q.stopCh:
			close(q.regCh)

			return

		case mrr := <-q.regCh:
			if mrr.sender == uuid.Nil {
				return
			}

			me := new(MsgEnvelope)
			me.Message = mrr.msg
			me.Registered = time.Now()
			me.sender = mrr.sender
			me.queue = q

			q.messages = append(q.messages, me)

			q.log.Debugw("message registered",
				"queue", q.name,
				"id", me.id,
				"key", me.Key)
		}
	}
}

// Count returns number of messages in the queue.
func (q *mQueue) Count() int {
	return len(q.messages)
}

// newQueue creates a new queue and returns a pointer on it.
//
// if there's nil logger given, error will be returned
func newQueue(
	id uuid.UUID,
	name string,
	log *zap.SugaredLogger) (*mQueue, error) {

	if log == nil {
		return nil, fmt.Errorf("nil logger given for queue %s", name)
	}

	if id == uuid.Nil {
		id = uuid.New()
	}

	if name == "" {
		name = "mQueue #" + id.String()
	}

	q := mQueue{
		id:         id,
		name:       name,
		messages:   make([]*MsgEnvelope, 0),
		lastReaded: make(map[uuid.UUID]int),
		log:        log,
		regCh:      make(chan msgRegRequest),
		stopCh:     make(chan struct{})}

	// start message registration procedure
	go q.regLoop()

	log.Debugw("new message queue is created", "name", q.name, "id", q.id)

	return &q, nil
}

// Stop stops the message registration cycle.
func (q *mQueue) Stop() {
	close(q.stopCh)
}

// PutMessages puts messages into the queue q.
//
// if there are no messages then error will be returned.
func (q *mQueue) PutMessages(sender uuid.UUID, msgs ...*Message) chan error {

	resChan := make(chan error, 1)

	if len(msgs) == 0 {
		q.log.Errorw("couldn't put empty message list on queue",
			"queue", q.name)

		resChan <- fmt.Errorf("couldn't put an empty messages "+
			"list into queue %s", q.name)
		return resChan

	}

	if sender == uuid.Nil {
		q.log.Errorw("sender isn't specified", "queue", q.name)

		resChan <- fmt.Errorf("sender isn't specified")
		return resChan
	}

	go func() {
		for _, m := range msgs {
			m := m
			q.regCh <- msgRegRequest{sender: sender, msg: *m}
			q.log.Debugw("message registration request sent",
				"queue", q.name,
				"msgID", m.id,
				"key", m.Key)
		}

		close(resChan)

	}()

	return resChan
}

// GetMessages returns a slice of messageEnvelopes
func (q *mQueue) GetMessages(
	reciever uuid.UUID,
	fromBegin bool) ([]MsgEnvelope, error) {

	if reciever == uuid.Nil {
		q.log.Errorw("reciever of messages isn't set", "queue", q.name)
		return nil,
			fmt.Errorf("reciever of message isn't set for queue %s", q.name)
	}

	from := q.lastReaded[reciever]
	if fromBegin {
		from = 0
	}
	var mes []MsgEnvelope
	for _, m := range q.messages[from:] {
		mes = append(mes, *m)
	}

	n := len(mes)

	q.lastReaded[reciever] = from + n

	q.log.Debugw("returnign messages", "count", n)

	return mes, nil
}

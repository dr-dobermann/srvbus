package ms

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// =============================================================================

// msgEnvelope holds the Message itself and the time when it was added
// to the queue and the Sender id who sent the Message into the queue.
type msgEnvelope struct {
	Message

	Registered time.Time
	sender     uuid.UUID
	queue      *MQueue
}

// msgRegRequest is used for message registration on server.
type msgRegRequest struct {
	sender uuid.UUID
	msg    Message
}

// Message Queue
type MQueue struct {
	id       uuid.UUID
	name     string
	messages []*msgEnvelope

	// lastReaded holds the last readed message id for the
	// particular reader
	lastReaded map[uuid.UUID]int

	log *zap.SugaredLogger

	regCh  chan msgRegRequest
	stopCh chan struct{}
}

// ID returns a queue's id
func (q MQueue) ID() uuid.UUID {
	return q.id
}

// Name returns queue's name
func (q MQueue) Name() string {
	return q.name
}

// regLoop gets message registration request from user q.PutMessages
func (q *MQueue) regLoop() {
	for {
		select {
		case <-q.stopCh:
			close(q.regCh)
			close(q.stopCh)

			return

		case mrr := <-q.regCh:
			me := new(msgEnvelope)
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

func (q *MQueue) Count() int {
	return len(q.messages)
}

// newQueue creates a new queue and returns a pointer on it.
//
// if there's nil logger given, error will be returned
func newQueue(
	id uuid.UUID,
	name string,
	log *zap.SugaredLogger) (*MQueue, error) {

	if log == nil {
		return nil, fmt.Errorf("nil logger given for queue %s", name)
	}

	if id == uuid.Nil {
		id = uuid.New()
	}

	if name == "" {
		name = "MQueue #" + id.String()
	}

	q := MQueue{
		id:         id,
		name:       name,
		messages:   make([]*msgEnvelope, 0),
		lastReaded: make(map[uuid.UUID]int),
		log:        log,
		regCh:      make(chan msgRegRequest),
		stopCh:     make(chan struct{})}

	// start message registration procedure
	go q.regLoop()

	log.Debugw("new message queue is created", "name", q.name, "id", q.id)

	return &q, nil
}

// PutMessages puts messages into the queue q.
//
// if there are no messages then error will be returned.
func (q *MQueue) PutMessages(sender uuid.UUID, msgs ...Message) chan error {

	resChan := make(chan error, 1)

	if len(msgs) == 0 {
		q.log.Errorw("couldn't put empty message list on queue",
			"queue", q.name)

		resChan <- fmt.Errorf("couldn't put an empty messages list into queue %s",
			q.name)
		return resChan

	}

	if sender == uuid.Nil {
		q.log.Errorw("sender isn't specified", "queue", q.name)

		resChan <- fmt.Errorf("sender isn't specified")
		return resChan
	}

	go func(sender uuid.UUID, msgs []Message) {
		for _, m := range msgs {
			m := m
			q.regCh <- msgRegRequest{sender: sender, msg: m}
			q.log.Debugw("message registration request sent",
				"queue", q.name,
				"msgID", m.id,
				"key", m.Key)
		}

		close(resChan)
	}(sender, msgs)

	return resChan
}

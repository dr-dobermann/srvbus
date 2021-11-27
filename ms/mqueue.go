package ms

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MessageEnvelope holds the Message itself and the time when it was added
// to the queue and the Sender id who sent the Message into the queue.
type MessageEnvelope struct {
	Message

	Registered time.Time
	Sender     uuid.UUID
}

// msgRegRequest is used for message registration on server.
type msgRegRequest struct {
	sender uuid.UUID
	msg    *Message
}

// queueMessagesRequest consist a single request for return
// messages to the receiver.
type queueMessagesRequest struct {
	receiver  uuid.UUID
	fromBegin bool
	messages  chan MessageEnvelope
}

// Message Queue
type mQueue struct {
	id       uuid.UUID
	name     string
	messages []*MessageEnvelope

	// lastReaded holds the last readed message id for the
	// particular reader
	lastReaded map[uuid.UUID]int

	log *zap.SugaredLogger

	// messages registration channel
	regCh chan msgRegRequest

	// queuue processing stop command channel
	stopCh chan struct{}

	// messages return channel
	mReqCh chan queueMessagesRequest

	// queue's messages number request channnel
	mCntCh chan int

	// number of messages in the queue
	cnt int
}

// count returns current number of the messages in the queue.
//
// if the queue loop is stopped, then -1 returned.
func (q mQueue) count() int {
	if c, ok := <-q.mCntCh; ok {
		return c
	}

	return -1
}

// isActive returns current queue's processing loop status.
func (q mQueue) isActive() bool {
	return q.count() >= 0
}

// ID returns a queue's id
func (q mQueue) ID() uuid.UUID {
	return q.id
}

// Name returns queue's name
func (q mQueue) Name() string {
	return q.name
}

// loop porcesses:
//   messages registration requests,
//   stop event
//   messages reading requests.
//
// loop also returns the cureent count of messages
// in the queue.
//
//nolint:cyclop
func (q *mQueue) loop() {
	for {
		select {
		case <-q.stopCh:
			close(q.regCh)
			close(q.mReqCh)
			close(q.mCntCh)

			return

		case mrr := <-q.regCh:
			if mrr.sender == uuid.Nil {
				return
			}

			me := new(MessageEnvelope)
			me.Message = *mrr.msg
			me.Registered = time.Now()
			me.Sender = mrr.sender

			q.messages = append(q.messages, me)

			q.cnt++

			q.log.Debugw("message registered",
				"queue", q.name,
				"id", me.id,
				"key", me.Key)

		case mReq := <-q.mReqCh:
			q.log.Debugw("got messages request",
				"queue", q.name,
				"receiver", mReq.receiver)

			from := q.lastReaded[mReq.receiver]
			if mReq.fromBegin {
				from = 0
			}

			mes := make([]MessageEnvelope, len(q.messages)-from)

			for i, me := range q.messages[from:] {
				mes[i] = *me
			}

			go func() {
				for _, me := range mes {
					me := me
					mReq.messages <- me
				}

				close(mReq.messages)
			}()

			q.log.Debugw("message request processing ended",
				"queue", q.name,
				"receiver", mReq.receiver,
				"messages number", len(mes))

			q.lastReaded[mReq.receiver] = from + len(mes)

		case q.mCntCh <- q.cnt: // send current messages count
			continue
		}
	}
}

// newQueue creates a new queue and returns a pointer on it.
//
// if there's nil logger given, error will be returned.
//
// newQueue runs the queue processing loop.
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
		messages:   make([]*MessageEnvelope, 0),
		lastReaded: make(map[uuid.UUID]int),
		log:        log,
		regCh:      make(chan msgRegRequest),
		stopCh:     make(chan struct{}),
		mReqCh:     make(chan queueMessagesRequest),
		mCntCh:     make(chan int)}

	// start processing loop
	go q.loop()

	log.Debugw("new message queue is created", "name", q.name, "id", q.id)

	return &q, nil
}

// Stop stops the message registration cycle.
func (q *mQueue) stop() {
	q.log.Infow("message stop", "queue", q.name)

	close(q.stopCh)
}

// putMessages puts messages into the queue q.
//
// if there are no messages then error will be returned.
func (q *mQueue) putMessages(
	ctx context.Context,
	sender uuid.UUID,
	msgs ...*Message) error {
	if !q.isActive() {
		q.log.Errorw("queue isn't processing requests",
			"queue", q.name)

		return fmt.Errorf("couldn't put messages into stopped queue %s",
			q.name)
	}

	if len(msgs) == 0 {
		q.log.Errorw("couldn't put empty message list on queue",
			"queue", q.name)

		return fmt.Errorf("couldn't put an empty messages "+
			"list into queue %s", q.name)
	}

	if sender == uuid.Nil {
		q.log.Errorw("sender isn't specified", "queue", q.name)

		return fmt.Errorf("sender isn't specified")
	}

	for _, m := range msgs {
		m := m
		select {
		case <-ctx.Done():
			close(q.stopCh)

			q.log.Error("context stopped for putting messages")

			return fmt.Errorf("put messages interrupted by context : %w",
				ctx.Err())

		case q.regCh <- msgRegRequest{sender: sender, msg: m}:
			q.log.Debugw("message registration request sent",
				"queue", q.name,
				"msgID", m.id,
				"key", m.Key)
		}
	}

	return nil
}

// g returns a slice of messageEnvelopes
func (q *mQueue) getMessages(
	ctx context.Context,
	receiver uuid.UUID,
	fromBegin bool) ([]MessageEnvelope, error) {
	if !q.isActive() {
		q.log.Errorw("queue isn't processing requests",
			"queue", q.name)

		return nil,
			fmt.Errorf("couldn't get messages from the stopped queue %s",
				q.name)
	}

	if receiver == uuid.Nil {
		q.log.Errorw("receiver of messages isn't set", "queue", q.name)

		return nil,
			fmt.Errorf("receiver of message isn't set for queue %s", q.name)
	}

	mes := []MessageEnvelope{}

	messages := make(chan MessageEnvelope)
	q.mReqCh <- queueMessagesRequest{receiver, fromBegin, messages}

	q.log.Debugw("start reading messages",
		"receiver", receiver,
		"queue", q.name,
		"from begin", fromBegin)

	for {
		select {
		case <-ctx.Done():
			close(q.stopCh)

			return nil,
				fmt.Errorf("message getting closed by context : %w",
					ctx.Err())

		case me, ok := <-messages:
			if !ok {
				q.log.Debugw("stop reading messages",
					"receiver", receiver,
					"queue", q.name,
					"count", len(mes))

				return mes, nil
			}

			q.log.Debugw("read message",
				"receiver", receiver,
				"queue", q.name,
				"msg ID", me.id,
				"msd Key", me.Key)

			mes = append(mes, me)
		}
	}
}

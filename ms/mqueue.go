package ms

import (
	"context"
	"fmt"
	"sync"
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

func (me MessageEnvelope) String() string {
	return fmt.Sprintf("MessageEnvelope(id : %v, registered at: %v from : %s "+
		"[key : \"%s\", value : \"%s\"])",
		me.ID, me.Registered, me.Sender, me.Name, string(me.Data()))
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
	sync.Mutex

	mSrv *MessageServer

	Name     string
	messages []*MessageEnvelope

	// lastReaded holds the last readed message id for the
	// particular reader
	lastReaded map[uuid.UUID]int

	log *zap.SugaredLogger

	// messages registration channel
	regCh chan msgRegRequest

	// messages return channel
	mReqCh chan queueMessagesRequest

	runned bool
}

// count returns current number of the messages in the queue.
//
// if the queue loop is stopped, then -1 returned.
func (q *mQueue) count() int {
	q.Lock()
	defer q.Unlock()

	return len(q.messages)
}

// isActive returns current queue's processing loop status.
func (q *mQueue) isActive() bool {
	q.Lock()
	defer q.Unlock()

	return q.runned
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
func (q *mQueue) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			q.Lock()
			q.runned = false
			q.Unlock()

			return

		case mrr := <-q.regCh:
			if mrr.sender == uuid.Nil {
				return
			}

			me := new(MessageEnvelope)
			me.Message = *mrr.msg
			me.Registered = time.Now()
			me.Sender = mrr.sender

			q.Lock()
			q.messages = append(q.messages, me)
			q.Unlock()

			q.mSrv.emitEvent("NEW_MSG_EVT",
				fmt.Sprintf(
					"{queue: \"%s\", msg_name: \"%s\", msg_sender: \"%v\"",
					q.Name, me.Name, me.Sender))

			q.log.Debugw("message registered",
				"msgID", me.Name,
				"key", me.Name)

		case mReq := <-q.mReqCh:
			q.log.Debugw("got messages request",
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
				"receiver", mReq.receiver,
				"messages number", len(mes))

			q.Lock()
			q.lastReaded[mReq.receiver] = from + len(mes)
			q.Unlock()
		}
	}
}

// newQueue creates a new queue and returns a pointer on it.
//
// if there's nil logger given, error will be returned.
//
// newQueue runs the queue processing loop.
func newQueue(
	ctx context.Context,
	name string,
	mSrv *MessageServer) (*mQueue, error) {

	q := mQueue{
		Name:       name,
		messages:   make([]*MessageEnvelope, 0),
		lastReaded: make(map[uuid.UUID]int),
		mSrv:       mSrv,
		log:        mSrv.log.Named(name),
		regCh:      make(chan msgRegRequest),
		mReqCh:     make(chan queueMessagesRequest)}

	// start processing loop
	q.Lock()
	q.runned = true
	q.Unlock()

	go q.loop(ctx)

	q.log.Debugw("new message queue is created",
		"queue", name)

	q.mSrv.emitEvent("NEW_QUEUE_EVT",
		fmt.Sprintf("{queue: \"%s\"", q.Name))

	return &q, nil
}

// putMessages puts messages into the queue q.
//
// if there are no messages then error will be returned.
func (q *mQueue) putMessages(
	ctx context.Context,
	sender uuid.UUID,
	msgs ...*Message) error {

	if !q.isActive() {
		return fmt.Errorf("couldn't put messages into stopped queue %s",
			q.Name)
	}

	go func() {
		for _, m := range msgs {
			m := m
			select {
			case <-ctx.Done():
				return

			case q.regCh <- msgRegRequest{sender: sender, msg: m}:
				q.log.Debugw("message registration request sent",
					"msgID", m.ID,
					"key", m.Name)
			}
		}
	}()

	return nil
}

// g returns a slice of messageEnvelopes
func (q *mQueue) getMessages(
	ctx context.Context,
	receiver uuid.UUID,
	fromBegin bool) (chan MessageEnvelope, error) {

	if !q.isActive() {
		return nil,
			fmt.Errorf("couldn't get messages from the stopped queue %s",
				q.Name)
	}
	messages := make(chan MessageEnvelope)

	q.log.Debugw("start reading messages",
		"receiver", receiver,
		"from begin", fromBegin)

	q.Lock()

	from := 0
	if !fromBegin {
		from = q.lastReaded[receiver]
	}

	res := append([]*MessageEnvelope{}, q.messages[from:]...)

	q.Unlock()

	go func() {
		c := len(res)
		for i, me := range res {
			me := me

			select {
			case <-ctx.Done():
				return

			case messages <- *me:
				q.log.Debug(fmt.Sprintf("message %d/%d sent", i+1, c),
					"receiver", receiver,
					"msgID", me.ID,
					"msd Key", me.Name)
			}
		}

		close(messages)
	}()

	return messages, nil
}

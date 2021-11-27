package ms

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

/// MessageServer holds all the Message Server data.
type MessageServer struct {
	sync.Mutex

	id   uuid.UUID
	Name string
	log  *zap.SugaredLogger
	ctx  context.Context

	queues map[string]*mQueue

	runned bool
}

// IsRunned returns the current running state of the MessageServer.
func (mSrv *MessageServer) IsRunned() bool {
	mSrv.Lock()
	defer mSrv.Unlock()

	return mSrv.runned
}

// QueueStat represent the statistics for a single queue.
type QueueStat struct {
	Name   string
	MCount int
}

// Queues returns a list of queues on the server
func (mSrv *MessageServer) Queues() []QueueStat {
	mSrv.Lock()
	defer mSrv.Unlock()

	ql := []QueueStat{}

	for n, q := range mSrv.queues {
		ql = append(ql, QueueStat{n, q.count()})
	}

	return ql
}

// New creates a new Message Server and returns its pointer.
//
// New starts internal go-routine to catch ctx.Done() signal
// and stops the server.
func New(
	ctx context.Context,
	id uuid.UUID,
	name string,
	log *zap.SugaredLogger) (*MessageServer, error) {
	if log == nil {
		return nil, fmt.Errorf("logger isn't present")
	}

	if id == uuid.Nil {
		id = uuid.New()
	}

	if name == "" {
		name = "MsgServer #" + id.String()
	}

	ms := &MessageServer{
		id:     id,
		Name:   name,
		log:    log,
		ctx:    ctx,
		queues: make(map[string]*mQueue)}

	log.Debugw("Message Server created",
		"name", ms.Name,
		"id", ms.id)

	go func() {
		<-ms.ctx.Done()

		ms.Lock()
		defer ms.Unlock()

		for _, q := range ms.queues {
			q.stop()
		}

		ms.runned = false
	}()

	return ms, nil
}

// PutMessages inserts messages into the queue.
//
// if queue name is empty error will be fired.
//
// if the queue isn't present on the server the new one queue will be
// created and added to the server.
// if some error occurred during the queue creation, the error will be
// returned.
func (mSrv *MessageServer) PutMessages(
	sender uuid.UUID,
	queue string,
	msgs ...*Message) error {
	if queue == "" {
		return fmt.Errorf("empty queue name for messages putting")
	}

	mSrv.Lock()
	defer mSrv.Unlock()

	_, ok := mSrv.queues[queue]
	if !ok {
		q, err := newQueue(uuid.New(), queue, mSrv.log)
		if err != nil {
			mSrv.log.Errorw("couldn't put messages",
				"queue", queue,
				"err", err.Error())

			return fmt.Errorf("put messages failde : %w", err)
		}

		mSrv.queues[queue] = q
	}

	q := mSrv.queues[queue]

	mSrv.log.Debugw("putting messages", "queue", q.name)

	return q.putMessages(mSrv.ctx, sender, msgs...)
}

// GetMessages reads all new messages form the queue and returns
// slice of their MessageEnvelopes.
//
// Receiver mustn't be nil and the queue should be on the server of
// error would be returned.
//
// If fromBegin is true, then all messages would be readed from the queue.
func (mSrv *MessageServer) GetMessages(
	receiver uuid.UUID,
	queue string,
	fromBegin bool) ([]MessageEnvelope, error) {
	q, ok := mSrv.queues[queue]
	if !ok {
		mSrv.log.Errorw("no queue", "queue", queue)

		return nil,
			fmt.Errorf("queue '%s' isn't found on the server '%s'",
				queue, mSrv.Name)
	}

	mSrv.log.Debugw("getting messages", "queue", q.name)

	return q.getMessages(mSrv.ctx, receiver, fromBegin)
}

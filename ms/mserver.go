package ms

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// =============================================================================
// Message Server

// MessageServer holds all the Message Server data.
type MessageServer struct {
	sync.Mutex

	id   uuid.UUID
	Name string
	log  *zap.SugaredLogger
	ctx  context.Context

	queues map[string]*mQueue
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

	return ms, nil
}

// PutMessages inserts messages into the queue.
//
// if queue name is empty error will be fired.
//
// if the queue isn't present on the server the new one queue will be
// created and added to the server.
// if some error occured during the queue creation, the error will be
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
			return fmt.Errorf("put messages failde : %w", err)
		}

		mSrv.queues[queue] = q
	}

	q := mSrv.queues[queue]

	if !q.isActive() {
		return fmt.Errorf("couldn't put messages into stopped queue %s",
			q.Name())
	}

	return q.putMessages(mSrv.ctx, sender, msgs...)
}

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

	return q.getMessages(mSrv.ctx, receiver, fromBegin)
}

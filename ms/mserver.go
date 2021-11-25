package ms

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// =============================================================================
// Message Server

// MessageServer holds all the Message Server data.
type MessageServer struct {
	id   uuid.UUID
	Name string
	log  *zap.SugaredLogger
	ctx  context.Context

	queues map[string]*mQueue
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

	q, ok := mSrv.queues[queue]
	if !ok {
		var err error
		q, err = newQueue(uuid.New(), queue, mSrv.log)
		if err != nil {
			return fmt.Errorf("put messages failde : %w", err)
		}
		mSrv.queues[queue] = q
	}

	return <-q.PutMessages(sender, msgs...)
}

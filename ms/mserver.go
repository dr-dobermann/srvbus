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

	queues map[string]*MQueue
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
		queues: make(map[string]*MQueue)}

	return ms, nil
}

func (mSrv *MessageServer) PutMessages(
	queue string,
	msgs ...*Message) chan error {

	var resChan chan error

	return resChan
}

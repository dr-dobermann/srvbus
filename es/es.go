package es

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type EventServer struct {
}

func New(id uuid.UUID, name string, log *zap.SugaredLogger) (*EventServer, error) {
	es := new(EventServer)

	return es, nil
}

func (eSrv *EventServer) Run(ctx context.Context) error {
	return fmt.Errorf("not implemented yet")
}

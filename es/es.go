// srvBus is a Service Providing Server developed to
// support project GoBPM.
//
// (c) 2021, Ruslan Gabitov a.k.a. dr-dobermann.
// Use of this source is governed by LGPL license that
// can be found in the LICENSE file.
//
/*
Package es is a part of the srvbus package. es consists of the
in-memory Events Server implementation.

Event Server provides the sub/pub model of the data exchange.
*/
package es

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// EventServer keeps the state of the event server.
type EventServer struct {
	ID   uuid.UUID
	Name string

	log *zap.SugaredLogger

	topics map[string]*Topic
}

// Creates a new EventServer.
func New(
	id uuid.UUID,
	name string,
	log *zap.SugaredLogger) (*EventServer, error) {

	if id == uuid.Nil {
		id = uuid.New()
	}

	if name == "" {
		name = "EventServer #" + id.String()
	}
	if log == nil {
		return nil,
			fmt.Errorf("log is absent for serverv %s # %v",
				name, id)
	}

	es := new(EventServer)
	es.Name = name
	es.ID = id
	es.log = log
	es.topics = make(map[string]*Topic)

	es.log.Infow("event server created",
		"eSrvID", es.ID,
		"name", es.Name)

	return es, nil
}

// Run starts the EventServer.
//
// To stope server use context's cancel function.
func (eSrv *EventServer) Run(ctx context.Context) error {
	eSrv.log.Infow("event server started",
		"eSrvID", eSrv.ID,
		"name", eSrv.Name)

	return fmt.Errorf("not implemented yet")
}

// srvBus is a Service Providing Server developed to
// support project GoBPM.
//
// (c) 2021, Ruslan Gabitov a.k.a. dr-dobermann.
// Use of this source is governed by LGPL license that
// can be found in the LICENSE file.

/*
Package s2 is a part of the srvbus package. s2 consists of the
in-memory Service Server implementation.

Service Server is registering, running and monitoring Services
needed for its users.
*/
package s2

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ServiceRunner present interface for service execution.
type ServiceRunner interface {
	Run(ctx context.Context) error
}

// ServiceServer holds the Service Server current state.
type ServiceServer struct {
	sync.Mutex

	ID   uuid.UUID
	Name string
	log  *zap.SugaredLogger
}

func New(
	id uuid.UUID,
	name string,
	log *zap.SugaredLogger) (*ServiceServer, error) {

	if log == nil {
		return nil, fmt.Errorf("logger isn't presented")
	}

	if id == uuid.Nil {
		id = uuid.New()
	}

	if name == "" {
		name = "ServiceServer #" + id.String()
	}

	sSrv := new(ServiceServer)
	sSrv.ID = id
	sSrv.Name = name
	sSrv.log = log

	sSrv.log.Debugw("new Service Server created",
		"id", sSrv.ID,
		"name", sSrv.Name)

	return sSrv, nil
}

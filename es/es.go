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

type EventServer struct {
}

func New(id uuid.UUID, name string, log *zap.SugaredLogger) (*EventServer, error) {
	es := new(EventServer)

	return es, nil
}

func (eSrv *EventServer) Run(ctx context.Context) error {
	return fmt.Errorf("not implemented yet")
}

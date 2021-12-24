package srvbus

import (
	"context"
	"fmt"
	"sync"

	"github.com/dr-dobermann/srvbus/es"
	"github.com/dr-dobermann/srvbus/internal/errs"
	"github.com/dr-dobermann/srvbus/ms"
	"github.com/dr-dobermann/srvbus/s2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type SBusErr struct {
	sbID uuid.UUID
	msg  string
	Err  error
}

func (sbErr SBusErr) Error() string {
	return fmt.Sprintf("SBErr[%v] %s: %w", sbErr.sbID, sbErr.msg, sbErr.Err)
}

type ServiceBus struct {
	sync.Mutex

	id uuid.UUID

	ctx context.Context

	log *zap.SugaredLogger

	eSrv *es.EventServer
	mSrv *ms.MessageServer
	sSrv *s2.ServiceServer

	topic string

	runned bool
}

func (sb *ServiceBus) ID() uuid.UUID {
	return sb.id
}

func (sb *ServiceBus) IsRunned() bool {
	sb.Lock()
	defer sb.Unlock()

	return sb.runned
}

func New(id uuid.UUID, log *zap.SugaredLogger) (*ServiceBus, error) {
	if id == uuid.Nil {
		id = uuid.New()
	}

	if log == nil {
		lg, err := zap.NewProduction()
		if err != nil {
			return nil, err
		}
		log = lg.Sugar()
	}

	sb := &ServiceBus{
		id:  id,
		log: log.Named("SB:  " + id.String()),
	}

	var err error

	sb.eSrv, err = es.New(uuid.New(), "SB_ES", log)
	if err != nil {
		return nil, SBusErr{id, "couldn't create an Event Server", err}
	}

	sb.mSrv, err = ms.New(uuid.New(), "SB_MS", log, sb.eSrv)
	if err != nil {
		return nil, SBusErr{id, "couldn't create a Message Server", err}
	}

	sb.sSrv, err = s2.New(uuid.New(), "SB_S2", log, sb.eSrv)
	if err != nil {
		return nil, SBusErr{id, "couldn't create a Service Server", err}
	}

	sb.log.Info("service bus created")

	return sb, nil
}

func (sb *ServiceBus) Run(ctx context.Context) error {
	if sb.IsRunned() {
		return errs.ErrAlreadyRunned
	}

	sb.ctx = ctx

	// run event server first so all others could emit events
	// just on time they runned
	if sb.eSrv == nil {
		return SBusErr{sb.id, "Event Server is absent", nil}
	}

	if err := sb.eSrv.Run(ctx, false); err != nil {
		return SBusErr{sb.id, "couldn't run an Event Server", err}
	}

	// run message server
	if sb.mSrv == nil {
		return SBusErr{sb.id, "Message Server is absent", nil}
	}

	if err := sb.mSrv.Run(ctx); err != nil {
		return SBusErr{sb.id, "couldn't run a Message Server", err}
	}

	// run service server
	if sb.sSrv == nil {
		return SBusErr{sb.id, "Service Server is absent", nil}
	}

	if err := sb.sSrv.Run(ctx); err != nil {
		return SBusErr{sb.id, "cannot run a Service Server", err}
	}

	sb.Lock()
	sb.runned = true
	sb.Unlock()

	go func() {
		<-ctx.Done()
		sb.Lock()
		sb.runned = false
		sb.Unlock()
	}()

	return nil
}

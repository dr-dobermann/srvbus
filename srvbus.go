package srvbus

import (
	"context"
	"fmt"
	"sync"

	"github.com/dr-dobermann/srvbus/es"
	"github.com/dr-dobermann/srvbus/ms"
	"github.com/dr-dobermann/srvbus/s2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

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
		log: log.Named("SB: " + id.String()),
	}

	sb.log.Info("service bus created")

	return sb, nil
}

func (sb *ServiceBus) Run(ctx context.Context) error {
	if sb.IsRunned() {
		sb.log.Warn("service bus already runned")
		return nil
	}

	sb.ctx = ctx

	// run or rerun EventServer
	if sb.eSrv == nil {
		eSrv, err := es.New(uuid.Nil, "SB_ES", sb.log)
		if err != nil {
			return fmt.Errorf("couldn't create EventServer: %v", err)
		}

		sb.eSrv = eSrv

		err = sb.eSrv.Run(ctx, false)
		if err != nil {
			return fmt.Errorf("coudln't run event server: %v", err)
		}

		topic := "/srvbus/" + sb.id.String()
		if !sb.eSrv.HasTopic(topic) {
			if err := sb.eSrv.AddTopicQueue(topic, "/"); err != nil {
				return fmt.Errorf("couldn't create SB topic '%s'", topic)
			}
		}

		sb.topic = topic
	}

	// run or rerun MessageServer
	if sb.mSrv == nil {
		mSrv, err := ms.New(uuid.Nil, "SB_MS", sb.log, sb.eSrv)
		if err != nil {
			return fmt.Errorf("couldn't create MessageServer: %v", err)
		}

		sb.mSrv = mSrv

		sb.mSrv.Run(ctx)
	}

	// run or rerun ServiceServer
	if sb.sSrv == nil {
		sSrv, err := s2.New(uuid.Nil, "SB_S2", sb.log, sb.eSrv)
		if err != nil {
			return fmt.Errorf("couldn't create ServiceServer: %v", err)
		}

		sb.sSrv = sSrv

		err = sb.sSrv.Run(ctx)
		if err != nil {
			return fmt.Errorf("couldn't run ServiceServer: %v", err)
		}
	}

	evt, err := es.NewEventWithString(
		"SB_STARTED_EVT", 
		fmt.Sprintf("{id: \"%v\"}", sb.id))
	if err != nil 
	sb.eSrv.AddEvent(sb.topic, )
	
	return nil
}

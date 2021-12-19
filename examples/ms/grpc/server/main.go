package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dr-dobermann/srvbus/api/grpc/ms_grpc"
	"github.com/dr-dobermann/srvbus/es"
	"github.com/dr-dobermann/srvbus/ms"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func main() {
	log, err := zap.NewProduction()
	if err != nil {
		panic("couldn't create log")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if ctx.Err() == nil {
			cancel()
		}
	}()

	eSrv, err := newES(ctx, log.Sugar())
	if err != nil {
		log.Sugar().Fatal("couldn't create an event server:", err)
	}

	mSrv, err := newMS(ctx, log.Sugar(), eSrv)
	if err != nil {
		log.Sugar().Fatal("couldn't create a message server:", err)
	}

	// Make a channel to listen for an interrupt or terminate signal from the OS.
	// Use a buffered channel because the signal package requires it.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	newMsgEvtCh := make(chan es.EventEnvelope)

	receiver := uuid.New()

	// run event listener
	go func() {
		log.Sugar().Info("event listening started")
		for {
			select {
			case <-ctx.Done():
				return

			case <-shutdown:
				cancel()

			case ee := <-newMsgEvtCh:
				log.Sugar().Infow("got new msg event",
					zap.String("name", ee.What().Name))

				mes, err := mSrv.GetMessages(receiver, "Test_Queue", false)
				if err != nil {
					log.Sugar().Warn("couldn't get messages: ", err)
					continue
				}

				for me := range mes {
					fmt.Println("  newMsg:", me.Name, "at: ",
						me.Registered, "[", string(me.Data()), "]")
				}
			}
		}
	}()

	eSrv.Subscribe(receiver, es.SubscrReq{
		Topic:     mSrv.ESTopic(),
		SubCh:     newMsgEvtCh,
		Recursive: false,
		Depth:     0,
		StartPos:  0,
		Filters: []es.Filter{
			es.WithName("NEW_MSG_EVT"),
			es.WithSubstr("queue: \"Test_Queue\"")},
	})

	// run gRPC server
	msGRPC, err := ms_grpc.New(mSrv, log.Sugar())
	if err != nil {
		log.Sugar().Fatal("couldn't create a gRPC server:", err)
	}

	err = msGRPC.Run(ctx, "localhost", "50051")
	if err != nil {
		log.Sugar().Fatal("error running gRPC server:", err)
	}

}

func newES(ctx context.Context,
	log *zap.SugaredLogger) (*es.EventServer, error) {

	eSrv, err := es.New(uuid.New(), "ES", log)
	if err != nil {
		return nil, fmt.Errorf("couldn't create an event server")
	}

	if err = eSrv.Run(ctx, false); err != nil {
		return nil, fmt.Errorf("couldn't run an event server")
	}

	return eSrv, nil
}

func newMS(
	ctx context.Context,
	log *zap.SugaredLogger,
	eSvr *es.EventServer) (*ms.MessageServer, error) {

	mSrv, err := ms.New(uuid.New(), "MS", log, eSvr)
	if err != nil {
		return nil, fmt.Errorf("couldn't create message server")
	}

	mSrv.Run(ctx)

	return mSrv, nil
}

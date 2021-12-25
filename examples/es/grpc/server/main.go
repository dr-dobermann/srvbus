package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/dr-dobermann/srvbus/api/grpc/esgrpc"
	"github.com/dr-dobermann/srvbus/es"
	"github.com/dr-dobermann/srvbus/internal/errs"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	host  = flag.String("host", "localhost", "grpc host name")
	port  = flag.Int("port", 50051, "grpc host port")
	debug = flag.Bool("debug", false, "run in debug mode")

	events_num = flag.Int("events_num", 5, "number of events to emit")
	emit_tout  = flag.Int("emit_timeout", 1, "timeout between events emitting")
)

func main() {
	flag.Parse()

	eSrv, err := getServer("ES_EX")
	if err != nil {
		panic(err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Make a channel to listen for an interrupt or terminate signal from the OS.
	// Use a buffered channel because the signal package requires it.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	time.AfterFunc(time.Second, func() {
		select {
		case <-ctx.Done():
			return

		case <-shutdown:
			cancel()
		}
	})

	err = eSrv.Run(ctx, es.NormalRun)
	if err != nil {
		panic(err.Error())
	}

	gES, err := esgrpc.New(eSrv, esgrpc.UseHostLogger)
	if err != nil {
		panic(err.Error())
	}

	runCh := make(chan error)

	go func() {
		runCh <- gES.Run(ctx, *host, strconv.Itoa(*port))
	}()

	err = emitEvents(eSrv)
	if err != nil {
		fmt.Println("event emitting error:", err)
	}

	err = <-runCh
	if err != nil {
		fmt.Println(err.Error())
	}

}

func getServer(name string) (*es.EventServer, error) {
	var (
		log *zap.Logger
		err error
	)

	if *debug {
		log, err = zap.NewDevelopment()
	} else {
		log, err = zap.NewProduction()
	}
	if err != nil {
		return nil, errs.ErrNoLogger
	}

	eSrv, err := es.New(uuid.New(), name, log.Sugar())
	if err != nil {
		return nil, fmt.Errorf("couldn't create an event server")
	}

	return eSrv, nil
}

func emitEvents(eSrv *es.EventServer) error {
	const (
		topic   = "/test_topic"
		evtName = "TEST_EVT"
	)

	senderID := uuid.New()

	if !eSrv.HasTopic(topic) && eSrv.AddTopic(topic, es.RootTopic) != nil {
		return fmt.Errorf("couldn't create topic '%s'", topic)
	}

	for i := 0; i < *events_num; i++ {
		evt, err := es.NewEventWithString(evtName, fmt.Sprintf("Test Event #%d", i))
		if err != nil {
			return fmt.Errorf("event #%d creating error: %v", i, err)
		}

		err = eSrv.AddEvent(topic, evt, senderID)
		if err != nil {
			return fmt.Errorf("adding event #%d error: %v", i, err)
		}

		time.Sleep(time.Duration(*emit_tout) * time.Second)
	}

	return nil
}

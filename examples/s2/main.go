package main

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/dr-dobermann/srvbus/ms"
	"github.com/dr-dobermann/srvbus/s2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func main() {
	// create logger
	log, err := zap.NewProduction()
	if err != nil {
		panic("couldn't create logger")
	}
	sl := log.Sugar()

	// create a new Service Server
	sSrv, err := s2.New(uuid.Nil, "s2 server", sl)
	if err != nil {
		panic("couldn't create a s3 server")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Service Server with cancelable context
	sSrv.Run(ctx)

	// create a new Message Server for support message interchange
	mSrv, err := ms.New(uuid.Nil, "msgSrv", sl)
	if err != nil {
		panic("couldn't create message server: " + err.Error())
	}

	// run Message Server
	mSrv.Run(ctx)

	// create channel to read messages from the Message Server
	mesCh := make(chan ms.MessageEnvelope)
	qn := "default queue"

	// create getMessage service on Service Server and run it
	// it should wait until queue qn apperas on the server and then
	// the Service start reading messages and sending them
	// into channel mesCh
	_, err = sSrv.AddService("Get service",
		s2.MustServiceRunner(
			s2.NewGetMessagesService(ctx, mSrv, qn, uuid.New(),
				false, true, 2*time.Second, 0,
				mesCh)), nil, true)
	if err != nil {
		panic("couldn't create getMessages service : " + err.Error())
	}

	// create and register on the Service Server new Service that put
	// messages into the queue qn.
	svcPut, err := sSrv.AddService("Put service",
		s2.MustServiceRunner(
			s2.NewPutMessagesService(ctx, mSrv, qn, uuid.New(),
				ms.GetMsg(uuid.New(),
					"greeting", bytes.NewBufferString("Hello Dober!")))),
		nil, false)

	if err != nil {
		panic("couldn't add putMessages service on server : " + err.Error())
	}

	// run the putMessages service
	if err = sSrv.ExecService(svcPut); err != nil {
		panic("couldn't exec put service : " + err.Error())
	}

	// read messages from the resulting service channel
	for me := range mesCh {
		fmt.Println("  got message:", me.String())
	}

	fmt.Println()
}

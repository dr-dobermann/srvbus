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
	log, err := zap.NewProduction()
	if err != nil {
		panic("couldn't create logger")
	}
	sl := log.Sugar()

	sSrv, err := s2.New(uuid.Nil, "s2 server", sl)
	if err != nil {
		panic("couldn't create a s3 server")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sSrv.Run(ctx)

	mSrv, err := ms.New(uuid.Nil, "msgSrv", sl)
	if err != nil {
		panic("couldn't create message server: " + err.Error())
	}
	mSrv.Run(ctx)

	mesCh := make(chan ms.MessageEnvelope)
	qn := "default queue"

	_, err = sSrv.AddService("Get service",
		s2.MustServiceRunner(
			s2.NewGetMessagesService(ctx, mSrv, qn, uuid.New(),
				false, true, 2*time.Second, 0,
				mesCh)), nil, true)
	if err != nil {
		panic("couldn't create getMessages service : " + err.Error())
	}

	svcPut, err := sSrv.AddService("Put service",
		s2.MustServiceRunner(
			s2.NewPutMessagesService(ctx, mSrv, qn, uuid.New(),
				ms.GetMsg(uuid.New(),
					"greeting", bytes.NewBufferString("Hello Dober!")))),
		nil, false)

	if err != nil {
		panic("couldn't add putMessages service on server : " + err.Error())
	}

	if err = sSrv.ExecService(svcPut); err != nil {
		panic("couldn't exec put service : " + err.Error())
	}

	for me := range mesCh {
		fmt.Println("  got message:", me.String())
	}

	fmt.Println()
}

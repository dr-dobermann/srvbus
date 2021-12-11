package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/dr-dobermann/srvbus/ms"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func main() {
	log, err := zap.NewDevelopment()
	if err != nil {
		panic("couldn't get a logger :" + err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())

	mSrv, err := ms.New(uuid.New(), "myserver", log.Sugar())
	if mSrv == nil || err != nil {
		panic("couldn't create a message server")
	}

	mSrv.Run(ctx)

	qn := "msg_queue"

	if err = mSrv.PutMessages(
		uuid.New(),
		qn,
		ms.MustMsg(
			ms.NewMsg(
				uuid.Nil,
				"greetings",
				strings.NewReader("Hello Dober!")))); err != nil {

		panic("couldn't store messages : " + err.Error())
	}

	mes, err := mSrv.GetMessages(uuid.New(), qn, false)
	if err != nil {
		panic("coudln't read a messages : " + err.Error())
	}

	i := 0
	for m := range mes {
		fmt.Println("#", i+1, "msg has key:'", m.Name,
			"' and data:[", string(m.Data()),
			"] received at", m.Registered,
			"from", m.Sender)
		i++
	}

	cancel()
}

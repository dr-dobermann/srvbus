package main

import (
	"context"
	"fmt"
	"log"
	"time"

	srvbus "github.com/dr-dobermann/go-srv-bus"
)

func main() {
	ms := srvbus.NewMessageServer()
	ms.Add("hello", "world!")

	srv := srvbus.NewServiceServer()

	srv.AddTask(srvbus.SrvMsgOutput, "Hello,", "world!")
	sid, err := srv.AddTask(srvbus.SrvGetMessage, ms, "hello", int64(1), 2)
	if err != nil {
		log.Fatal("couldn't add GetMessage service due to", err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv.Run(ctx)
	time.Sleep(5 * time.Second)
	ms.Add("hello", "dober!")
	time.Sleep(5 * time.Second)
	results, err := srv.GetResults(sid)
	if err != nil {
		log.Fatal("couldn't get results of Get Message service ", err)
	}
	fmt.Println("Got message(s):")
	for _, m := range results {
		ms := m.(srvbus.Message)
		fmt.Printf("%v\n", ms.String())
	}

	srv.ListServices()

	panic("stack as it is...")
}

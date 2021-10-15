package main

import (
	"context"
	"fmt"
	"log"
	"time"

	srvbus "github.com/dr-dobermann/go-srv-bus"
)

func main() {
	ms := srvbus.NewMessageServer("my_test_server")
	srv := srvbus.NewServiceServer()

	srv.AddTask(srvbus.SrvOutput, "Hello,", "world!")
	sid, err := srv.AddTask(srvbus.SrvGetMessages, srvbus.MsgServerDef{ms, "hello", 1}, int64(2))
	srv.AddTask(srvbus.SrvPutMessages, srvbus.MsgServerDef{ms, "hello", 1}, "sweet", "dober")
	if err != nil {
		log.Fatal("couldn't add GetMessage service due to", err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv.Run(ctx)
	time.Sleep(10 * time.Second)
	results, err := srv.GetResults(sid, false)
	if err != nil {
		log.Fatal("couldn't get results of Get Message service ", err)
	}
	fmt.Println("Got message(s):")
	for _, m := range results {
		ms := m.(srvbus.Message)
		fmt.Printf("%v\n", ms.String())
	}

	srv.ListServices()
}

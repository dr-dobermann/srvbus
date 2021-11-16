package main

import (
	"context"
	"fmt"
	"time"

	"github.com/dr-dobermann/srvbus"
	"github.com/google/uuid"
)

func main() {
	ms := srvbus.NewMessageServer("my_test_server")
	srv := srvbus.NewServiceServer()

	printRes(srv.AddTask(srvbus.SrvOutput, "Hello,", "world!"))
	printRes(srv.AddTask(srvbus.SrvGetMessages, srvbus.MsgServerDef{ms, "hello", 1}, int64(2)))
	printRes(srv.AddTask(srvbus.SrvPutMessages, srvbus.MsgServerDef{ms, "hello", 1}, "sweet", "dober"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv.Run(ctx)
	time.Sleep(10 * time.Second)

	for _, si := range srv.ListServices() {

		results, err := srv.GetResults(si.Id, false)
		if err != nil {
			fmt.Printf("couldn't get results for task %s : %v", si.Id, err)
			continue
		}

		for _, m := range results {
			ms := m.(srvbus.Message)
			fmt.Printf("%v\n", ms.String())
		}
	}
}

func printRes(id uuid.UUID, err error) {
	if err == nil {
		fmt.Println("Task", id, "successfully added")
		return
	}

	fmt.Printf("Adding task %v caused error %v", id, err)
}

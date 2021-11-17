package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/dr-dobermann/srvbus/msgsrv"
)

func main() {
	ms := msgsrv.NewMessageServer("myserver")
	if ms == nil {
		panic("couldn't create a message server")
	}

	qn := "msg_queue"
	// trying to retrieve message
	go func() {
		for {
			if ms.HasQueue(qn) {
				break
			}
			fmt.Println("no queue", qn, "on server", ms.Name, ". Wait...")
			time.Sleep(100 * time.Millisecond)
		}

		mm, err := ms.GetMesages(qn)
		if err != nil {
			panic(fmt.Sprintf("couldn't retrieve messages "+
				"from queue %s on server %s : %v",
				qn, ms.Name, err))
		}
		for _, m := range mm {
			fmt.Printf("Got message :\n  registered at: %v\n"+
				"  key: %s\n  value: %v (%s)\n", m.RegTime, m.Key, m.Data, string(m.Data))
		}
	}()

	// write message to server
	time.AfterFunc(
		1*time.Second,
		func() {
			fmt.Println("Putting message")
			ms.PutMessages(qn, *msgsrv.GetMsg("greetings", strings.NewReader("Hello Dober!")))
		})

	fmt.Println("Closing...")

	time.Sleep(2 * time.Second)
}

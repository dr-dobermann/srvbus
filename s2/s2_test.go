package s2

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/matryer/is"
)

func TestOutputService(t *testing.T) {
	is := is.New(t)

	// check nil Writer
	if _, err := NewOutputSvc("Nil Writer", nil, "test"); err == nil {
		t.Fatal("Nil Writer accepted")
	}

	var buf bytes.Buffer
	oss, err := NewOutputSvc("Output Service", &buf, "Hello world!", " ", "Hello Dober!")
	is.NoErr(err)
	if oss == nil {
		t.Error("Couldn't create Output Service")
	}

	is.NoErr(oss.Run(context.Background()))

	is.Equal(buf.String(), "Hello world! Hello Dober!")
}

func TestServiceServer(t *testing.T) {
	is := is.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := NewServiceServer("test_server", ctx)
	if srv == nil {
		t.Fatal("Couldn't create a server")
	}

	// check nil service addition
	if err := srv.AddService(nil); err == nil {
		t.Fatal("Nil Service added")
	}

	var buf bytes.Buffer

	is.NoErr(srv.AddService(GetOutputSvc("Output Service", &buf, "Hello world! ", "Hello Dober!")))
	if len(srv.services) != 1 {
		t.Error("Invalid services count", len(srv.services))
	}

	//buf1 := bytes.NewBuffer(nil)
	is.NoErr(srv.AddService(GetOutputSvc("Output Service 2", os.Stderr, "Hello again Dober!\n")))

	srv.Start(ctx)

	fmt.Println("Pause for 3 seconds...")
	time.Sleep(3 * time.Second)

	stat := srv.Stats()
	fmt.Println(stat)

	is.Equal(srv.state, SrvExecutingServices)
	is.Equal(buf.String(), "Hello world! Hello Dober!")

}

// func TestMessageServices(t *testing.T) {

// 	ctx, cancelCtx := context.WithCancel(context.Background())
// 	defer cancelCtx()

// 	ms := msgsrv.NewMessageServer("test_message_server")

// 	pms, err := NewPutMessageSvc("PutMsg Service", ms, *msgsrv.GetMsg("test_msg"), )
// }

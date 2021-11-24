package s2

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/dr-dobermann/srvbus/msgsrv"
	"github.com/matryer/is"
)

func TestMessage(t *testing.T) {
	is := is.New(t)

	s := "This is a test string"
	m := msgsrv.MustGetMsg("test", bytes.NewBufferString(s))

	var mv bytes.Buffer
	io.Copy(&mv, m)

	is.Equal(mv.String(), s)
}

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

	srv := NewServiceServer(ctx, "test_server")
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
	time.Sleep(3 * time.Second)

	stat := srv.Stats()
	fmt.Println(stat)

	is.Equal(buf.String(), "Hello world! Hello Dober!")

}

func TestMessageServices(t *testing.T) {
	is := is.New(t)

	ms := msgsrv.NewMessageServer("test_message_server")
	qn := "test_queue"
	m := []struct{ k, v string }{
		{k: "test_msg", v: "Hello Dober!"},
	}

	pms, err := NewPutMessagesSvc(
		"PutMsg Service",
		ms,
		qn,
		msgsrv.MustGetMsg(m[0].k, bytes.NewBufferString(m[0].v)))

	is.NoErr(err)

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	srv := NewServiceServer(ctx, "test_message_service_server")

	is.NoErr(srv.AddService(pms))

	gms := GetGetMessagesSvc("Get Messages Service", ms, qn, 2, 1, 0)
	is.NoErr(srv.AddService(gms))

	is.NoErr(srv.Start(ctx))

	msgPrinter := func(res <-chan interface{}) error {
		fmt.Println("Printing gathered messages:")
		i := 0
		for mc := range res {
			msg, ok := mc.(msgsrv.Message)
			if !ok {
				t.Error("Couldn't get a message from", mc)
				continue
			}
			// printing messages
			fmt.Println("@", msg.RegTime, ": key =", msg.Key, ",  value =", string(msg.Data()))

			// test message values aginst it's origin
			is.Equal(msg.Key, m[i].k)
			is.Equal(string(msg.Data()), m[i].v)

			i++
		}

		return nil
	}

	is.NoErr(gms.UploadResults(ctx, msgPrinter))

	fmt.Println(srv.Stats())
}
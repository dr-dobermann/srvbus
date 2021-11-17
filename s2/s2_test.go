package s2

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/matryer/is"
)

func TestOutputService(t *testing.T) {
	is := is.New(t)

	// check nil Writer
	if _, err := NewOutputService("Nil Writer", nil, "test"); err == nil {
		t.Fatal("Nil Writer accepted")
	}

	var buf bytes.Buffer
	oss, err := NewOutputService("Output Service", &buf, "Hello world!", " ", "Hello Dober!")
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

	is.NoErr(srv.AddService(GetOutputService("Output Service", &buf, "Hello world! ", "Hello Dober! ")))
	if len(srv.services) != 1 {
		t.Error("Invalid services count", len(srv.services))
	}

	is.NoErr(srv.AddService(GetOutputService("Output Service 2", &buf, "Hello again Dober!")))

	srv.Start(ctx)
	is.Equal(srv.state, SrvExecutingServices)
	is.Equal(buf.String(), "Hello world! Hello Dober! Hello again Dober!")

	stat := srv.Stats()

	fmt.Println(stat)
}

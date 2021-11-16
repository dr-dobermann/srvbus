package s2

import (
	"testing"

	"github.com/matryer/is"
)

func TestServiceServer(t *testing.T) {
	is := is.New(t)

	srv := NewServiceServer("test_server")
	if srv == nil {
		t.Fatal("Couldn't create a server")
	}

	os, err := NewOutputService("Output Service", "Hello, world!\n", "Hello, dober!\n", 23, "\n")
	is.NoErr(err)
	if os == nil {
		t.Error("Couldn't create Output Service")
	}
}

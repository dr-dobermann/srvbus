package es

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/matryer/is"
)

func TestEvntServerCreation(t *testing.T) {
	is := is.New(t)

	// check server creation with nil log
	_, err := New(uuid.Nil, "", nil)
	is.True(err != nil)

	// getServer is in topic_test.go
	// it creates a log and returns EventServer with
	// it. If there is any error it makes test falil.
	eSrv := getServer(uuid.New(), "ESTest", t)
	is.True(eSrv != nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	is.NoErr(eSrv.Run(ctx, false))
}

func TestAddingEvents(t *testing.T) {
	is := is.New(t)

	eSrv := getServer(uuid.New(), "ESTest", t)
	is.True(eSrv != nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// add event on non-runned server
	is.True(
		eSrv.AddEvent("/main",
			MustEvent(NewEventWithString("test_event", "test_event fired")),
			uuid.New()) != nil)

	is.NoErr(eSrv.Run(ctx, true))

	is.NoErr(eSrv.AddTopicQueue("/main/subtopic/subsubtopic", "/"))

	// event with no sender
	is.True(
		eSrv.AddEvent("/main",
			MustEvent(NewEventWithString("test_event", "test_event fired")),
			uuid.Nil) != nil)

	is.NoErr(
		eSrv.AddEvent("/main",
			MustEvent(NewEventWithString("test_event", "test_event fired")),
			uuid.New()))
}

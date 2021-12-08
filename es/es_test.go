package es

import (
	"context"
	"testing"
	"time"

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

	// add event on non-runned server
	is.True(
		eSrv.AddEvent("/main",
			MustEvent(NewEventWithString("test_event", "test_event fired")),
			uuid.New()) != nil)

	is.NoErr(eSrv.Run(ctx, true))

	is.NoErr(eSrv.AddTopicQueue("/main/subtopic/subsubtopic", "/"))

	// empty event registration
	is.True(
		eSrv.AddEvent("/main",
			nil,
			uuid.New()) != nil)

	// event with no sender
	is.True(
		eSrv.AddEvent("/main",
			MustEvent(NewEventWithString("test_event", "test_event fired")),
			uuid.Nil) != nil)

	// event with invalid topic
	is.True(
		eSrv.AddEvent("/mani",
			MustEvent(NewEventWithString("test_event", "test_event fired")),
			uuid.Nil) != nil)

	err := eSrv.AddEvent("/main",
		MustEvent(NewEventWithString("good_test_event",
			"good_test_event fired")),
		uuid.New())
	is.NoErr(err)

	// check if the event is in

	time.AfterFunc(5*time.Second, cancel)
}

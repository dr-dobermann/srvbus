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

	eName := "TEST_EVT"
	// add event on non-runned server
	is.True(
		eSrv.AddEvent("/main",
			MustEvent(NewEventWithString(eName, "test_event fired")),
			uuid.New()) != nil)

	is.NoErr(eSrv.Run(ctx, true))

	is.NoErr(eSrv.AddTopicQueue("/main/subtopic/subsubtopic", "/"))

	sender := uuid.New()

	// =========================================
	//
	// testing invalid cases
	test_event := MustEvent(
		NewEventWithString(eName, "test_event fired"))
	err_cases := []struct {
		test_name string
		topic     string
		evt       *Event
		sender    uuid.UUID
	}{
		{"empty event registration", "/main", nil, sender},
		{"event with no sender", "/main", test_event, uuid.Nil},
		{"event with invalid topic", "/mani", test_event, sender},
		{"event with invalid subtopic", "/main/ssstopic", test_event, sender},
	}

	for _, ecase := range err_cases {
		t.Run(ecase.test_name, func(t *testing.T) {
			is.True(
				eSrv.AddEvent(ecase.topic, ecase.evt, ecase.sender) != nil)
		})
	}

	// ===========================================
	//
	// testing adding event on any level of topic
	events := []struct{ topic, event string }{
		{"/main", "MAIN_EVT"},
		{"/main/subtopic", "SUBTOPIC_EVT"},
		{"/main/subtopic/subsubtopic", "SSTOPIC_EVT"}}

	for _, e := range events {
		t.Run("add_evt_to_"+e.topic, func(t *testing.T) {
			is.NoErr(
				eSrv.AddEvent("/main/subtopic/",
					MustEvent(NewEventWithString(e.event, e.event+" fired")),
					sender))
		})
	}

	// wait for events registration
	time.Sleep(2 * time.Second)

	// stop the server
	cancel()
}

func TestTopicsManagement(t *testing.T) {
	is := is.New(t)

	eSrv := getServer(uuid.New(), "ESTest", t)
	is.True(eSrv != nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	is.NoErr(eSrv.Run(ctx, true))

	tn := "/main/subtopic/subsubtopic"
	tn1 := "st/sst/ssst"

	is.NoErr(eSrv.AddTopicQueue(tn, ""))
	is.NoErr(eSrv.AddTopicQueue(tn1, "/main"))

	is.True(eSrv.HasTopic(tn))
	is.True(eSrv.HasTopic("/main/" + tn1))

	is.NoErr(eSrv.RemoveTopic("/main/st/sst", RECURSIVE))
	is.True(!eSrv.HasTopic("/main/st/sst"))

	is.True(eSrv.RemoveTopic("/main", ONLY_ONE_TOPIC) != nil)
}

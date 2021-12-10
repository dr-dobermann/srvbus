package es

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/matryer/is"
)

func TestSubscriptions(t *testing.T) {
	is := is.New(t)

	// create server
	eSrv := getServer(uuid.New(), "eSrv:Test", t)
	is.True(eSrv != nil)

	// run it
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	is.NoErr(eSrv.Run(ctx, false))

	// add some topics
	mnt := "/main"
	st := "/main/subtopic"
	sst := "/main/subtopic/subsubtopic"
	is.NoErr(eSrv.AddTopicQueue(sst, ""))

	subscriber := uuid.New()

	subCh := make(chan EventEnvelope)
	defer close(subCh)

	// check for invalid subscriptions
	err_subs := map[string]struct {
		subscr uuid.UUID
		sr     SubscrReq
	}{
		"no_subscriber": {uuid.Nil,
			SubscrReq{"/main", subCh, ONLY_ONE_TOPIC, 0, 0, ALL_EVENTS}},
		"no_topic": {subscriber,
			SubscrReq{"/mani", subCh, ONLY_ONE_TOPIC, 0, 0, ALL_EVENTS}},
		"no_channel": {subscriber,
			SubscrReq{"/main", nil, ONLY_ONE_TOPIC, 0, 0, ALL_EVENTS}}}
	for tn, s := range err_subs {
		t.Run(tn, func(t *testing.T) {
			is.True(eSrv.Subscribe(s.subscr, s.sr) != nil)
		})
	}

	// add subscriptions
	subs := []SubscrReq{
		{mnt, subCh, RECURSIVE, 1, FROM_BEGIN, ALL_EVENTS},
		{sst,
			subCh, ONLY_ONE_TOPIC, 0, ONLY_NEW_EVENTS,
			[]Filter{WithSubName("GREETING")}},
	}
	is.NoErr(eSrv.Subscribe(subscriber, subs...))

	// emit some events to subscribed topics
	sender := subscriber // it's possible than sender is
	// the same as the subscriber
	events := []struct {
		topic string
		evt   *Event
	}{
		{sst, MustEvent(NewEventWithString("TEST_EVT", "Shouldn't pass filter"))},
		{sst, MustEvent(NewEventWithString("GREETINGS", "Hello Dr.Dobermann!"))},
		{mnt, MustEvent(NewEventWithString("EVR_GRTNG", "Hello everybody!"))},
		{st, MustEvent(NewEventWithString("WRLD_GRTNG", "Hello world!"))},
	}

	// run event reciever
	// we shouldn't get only one event (0)
	validEvents := []*Event{events[1].evt, events[2].evt, events[3].evt}
	m := sync.Mutex{}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return

			case ee, next := <-subCh:
				if !next {
					return
				}

				fmt.Printf(" ==> [%s] %v @%v : %s\n",
					ee.Topic, ee.Publisher, ee.RegAt, ee.event.String())

				// check event in validEvents
				pos := -1
				for i, e := range validEvents {
					if e.Name == ee.event.Name {
						pos = i
					}
				}

				if pos == -1 {
					panic("unexpected event " + ee.event.Name)
				}

				m.Lock()
				validEvents = append(validEvents[:pos], validEvents[pos+1:]...)
				m.Unlock()
			}
		}
	}()

	for _, te := range events {
		is.NoErr(eSrv.AddEvent(te.topic, te.evt, sender))
	}

	time.Sleep(2 * time.Second)

	// all events should be deleted when they are readed from the channel.
	m.Lock()
	is.Equal(len(validEvents), 0)
	m.Unlock()

	// unsubscribe from topics "/main"
	is.NoErr(eSrv.UnSubscribe(subscriber, mnt))

	// emit events for cancelled subscriptions
	is.NoErr(eSrv.AddEvent(events[2].topic, events[2].evt, sender))

	time.Sleep(2 * time.Second)
}

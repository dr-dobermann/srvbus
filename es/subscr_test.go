package es

import (
	"context"
	"fmt"
	"testing"

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
	is.NoErr(eSrv.AddTopicQueue("/main/subtopic/subsubtopic", ""))

	subscriber := uuid.New()
	subCh := make(chan EventEnvelope)

	var filters []Filter

	// check for invalid subscriptions
	err_subs := map[string]struct {
		subscr uuid.UUID
		sr     SubscrReq
	}{
		"no_subscriber": {uuid.Nil,
			SubscrReq{"/main", subCh, false, 0, 0, filters}},
		"no_topic": {subscriber,
			SubscrReq{"/mani", subCh, false, 0, 0, filters}},
		"no_channel": {subscriber,
			SubscrReq{"/main", nil, false, 0, 0, filters}}}
	for tn, s := range err_subs {
		t.Run(tn, func(t *testing.T) {
			is.True(eSrv.Subscribe(s.subscr, s.sr) != nil)
		})
	}

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
			}
		}
	}()

	// add subscriptions
	subs := []SubscrReq{
		{"/main", subCh, true, 1, 0, filters},
		{"/main/subtopic/subsubtopic",
			subCh, false, 0, 0, []Filter{WithSubName("DOBER")}},
	}
	is.NoErr(eSrv.Subscribe(subscriber, subs...))

	// emit some events to subscribed topics

	// unsubscribe from some events

	// emit events for cancelled subscriptions
}

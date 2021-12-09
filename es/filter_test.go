package es

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/matryer/is"
)

func TestFilters(t *testing.T) {
	is := is.New(t)

	testEnvelope := EventEnvelope{
		event: MustEvent(NewEventWithString(
			"TEST_EVENT",
			"this is a test event")),
		Topic:     "/main",
		Publisher: uuid.New(),
		RegAt:     time.Now(),
		Index:     0,
	}

	errFilters := []Filter{
		WithName("TEST_EVT"),
		WithSubName("test"),
		WithSubData([]byte("hello")),
		WithSubstr("hello")}

	goodFilters := []Filter{
		WithName("TEST_EVENT"),
		WithSubName("EVENT"),
		WithSubData([]byte("test event")),
		WithSubstr("is a test")}

	sub := subscription{
		subscriber: uuid.New(),
		eCh:        make(chan EventEnvelope),
	}

	testData := map[string]struct {
		filters []Filter
		res     *EventEnvelope
	}{
		"with_error_filters": {errFilters, nil},
		"with_good_filters":  {goodFilters, &testEnvelope}}

	for tn, d := range testData {
		t.Run(tn, func(t *testing.T) {
			sub.filters = d.filters

			is.Equal(sub.filter(&testEnvelope), d.res)
		})
	}

}

package es

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// Filter provides simple test for byte stream
type Filter interface {
	check(name string, data []byte) bool
}

// filterFunc checks bytestream for internal conditions.
type filterFunc func(name string, data []byte) bool

// Implementation of Filter interface for filterFunc.
func (f filterFunc) check(name string, data []byte) bool {
	return f(name, data)
}

const (
	FROM_BEGIN      int = 0
	ONLY_NEW_EVENTS int = -1

	RECURSIVE      bool = true
	ONLY_ONE_TOPIC bool = false
)

var ALL_EVENTS []Filter = []Filter{}

// SubscrReq consists of
type SubscrReq struct {
	Topic     string
	SubCh     chan EventEnvelope
	Recursive bool
	Depth     uint
	StartPos  int
	Filters   []Filter
}

// checking subscription request obligatory fields
func (sr SubscrReq) check() error {
	t := strings.Trim(sr.Topic, " ")
	if t == "" {
		return fmt.Errorf("emtpy topic")
	}

	if sr.SubCh == nil {
		return fmt.Errorf("no subscription channel")
	}

	return nil
}

// subscription keeps status for one Subscriber.
type subscription struct {
	sync.Mutex

	// subscriber id
	subscriber uuid.UUID

	// subscription request
	subReq string

	// channel for subscribed topics
	eCh chan EventEnvelope

	// last readed index of event in the related topic
	//
	// it used only if filterCond is not set (!nil)
	lastReaded int

	// event filter
	filters []Filter
}

// sendding single event into subscription
//
// there are two way of event sending:
//   the first one used if the subscription has filters
//     and subscriber gets only events which comply those filters'
//     conditions
//   the second way is the sequental sending of all events
//     putting into the topic
func (s *subscription) sendEvent(ctx context.Context,
	ee *EventEnvelope,
	pos int) {

	// check if there are any filter and event comply its conditions.
	if s.filters != nil && len(s.filters) > 0 {
		if ee := s.filter(ee); ee != nil {
			// try to send event or stop on context's cancel
			select {
			case s.eCh <- *ee:
			case <-ctx.Done():
			}
		}

		return
	}

	// sequental sending of events
	for {
		// check context cancelling
		select {
		case <-ctx.Done():
			return
		default:
		}

		s.Lock()
		// if sender wants to get first event in the t.events
		//
		// or
		//
		// wait until sender accepts all the previous events
		// and then send the event into the output channel
		if (s.lastReaded == 0 && pos == 0) ||
			(s.lastReaded+1 == pos) {
			s.eCh <- *ee

			// set lastRead according to event position in
			// the t.events
			s.lastReaded = pos
			s.Unlock()

			return
		}

		s.Unlock()
	}
}

// filter checks if the event comply to filterConditions.
//
// if checking passed, filter returns given EventEnvelope
// and nil otherwise.
func (s *subscription) filter(ee *EventEnvelope) *EventEnvelope {
	// parallel filtration
	failState := false
	filterFail := &failState

	wg := sync.WaitGroup{}
	wg.Add(len(s.filters))

	for _, f := range s.filters {
		f := f
		go func() {
			s.Lock()
			defer s.Unlock()
			res := f.check(ee.event.Name, ee.event.Data())
			if !res {

				*filterFail = true
			}
			wg.Done()
		}()
	}

	wg.Wait()

	if *filterFail {
		return nil
	}

	// // sequental filtration
	// for _, f := range s.filters {
	// 	if !f.check(ee.event.Name, ee.event.Data()) {
	// 		return nil
	// 	}
	// }

	return ee
}

// =============================================================================
//                        Filters
//

// Has returns filter which checks if name is equal with event name.
func WithName(name string) Filter {
	filter := func(n string, _ []byte) bool {
		return n == name
	}

	return filterFunc(filter)
}

// WhithSubMame checks if event name has str in it.
func WithSubName(str string) Filter {
	filter := func(n string, _ []byte) bool {
		return strings.Contains(n, str)
	}

	return filterFunc(filter)
}

// WithSubData checks if Event data consists dat in it.
func WithSubData(dat []byte) Filter {
	filter := func(_ string, data []byte) bool {
		return bytes.Contains(data, dat)
	}

	return filterFunc(filter)
}

// WithSubstr checks if Event data consists str in it.
func WithSubstr(str string) Filter {
	filter := func(_ string, data []byte) bool {
		return bytes.Contains(data, []byte(str))
	}

	return filterFunc(filter)
}

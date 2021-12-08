package es

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type EventEnvelope struct {
	event *Event

	Topic     string
	Publisher uuid.UUID
	RegAt     time.Time

	// event index in the topic storage
	Index int
}

//
type eventFilter struct {
	name string
	data string
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
	filterCond *eventFilter
}

// filter checks if the event comply to filterConditions.
//
// if checking passed, filter returns given EventEnvelope
// and nil otherwise.
func (s *subscription) filter(ee *EventEnvelope) *EventEnvelope {

	return ee
}

// Topic keep state of a single topic.
type Topic struct {
	sync.Mutex

	// the EventServer the topic is belong to
	eServer *EventServer

	// full topic name
	fullName string

	// short name
	name string

	// events queue
	events []EventEnvelope

	// nested subtopics
	subtopics map[string]*Topic

	// incoming channel
	inCh chan EventEnvelope

	// subscribers for the queue
	subs map[uuid.UUID]*subscription

	// running flag
	runned bool

	// runned context
	ctx context.Context
}

// isRunned returns the running status of the topic
func (t *Topic) isRunned() bool {
	t.Lock()
	defer t.Unlock()

	return t.runned
}

// addSubtopic adds a new subtopic if there is no duplicates.
//
// Function gets a new subtopic name and slice of the base topics.
// This slice doesnt consist of t.Name only the topics which are under
// the t.
func (t *Topic) addSubtopic(name string, base []string) error {
	t.Lock()
	defer t.Unlock()

	// if current Topic is the last one in the base,
	// add new topic to it and return
	if len(base) == 0 {
		if _, ok := t.subtopics[name]; ok {
			return newESErr(
				t.eServer,
				nil,
				"topic '%s' already has subtopic '%s'",
				t.fullName, name)
		}

		nt := &Topic{
			eServer:   t.eServer,
			fullName:  t.fullName + "/" + name,
			name:      name,
			events:    []EventEnvelope{},
			subtopics: map[string]*Topic{},
			inCh:      make(chan EventEnvelope),
			subs:      map[uuid.UUID]*subscription{}}
		t.subtopics[name] = nt

		if t.runned {
			nt.run(t.ctx)
		}

		t.eServer.log.Debugw("subtopic added",
			"eSrvID", t.eServer.ID,
			"eSrvName", t.eServer.Name,
			"subtopic", name,
			"branch", t.fullName)

		return nil
	}

	// check if t has subtopic named as the first out of base.
	// if it exists, add new topic as its subtopic.
	st, ok := t.subtopics[base[0]]
	if !ok {
		return newESErr(t.eServer, nil,
			"topic '%s' has no subtopic '%s'", t.fullName, base[0])
	}

	return st.addSubtopic(name, base[1:])
}

// hasSubtopic checks if topics are existed in the topic.
//
// if the given topics has subtopics, they would be checked
// over recursive calls of its hasSubtopic.
func (t *Topic) hasSubtopic(topics []string) (*Topic, bool) {
	t.Lock()
	defer t.Unlock()

	// check if t owns the first topic in the topics
	st, ok := t.subtopics[topics[0]]
	if !ok {
		return nil, false
	}

	// if there are subtopics, check them in the topic found earlier
	if len(topics) > 1 {
		return st.hasSubtopic(topics[1:])
	}

	return st, true
}

// run starts topic execution
func (t *Topic) run(ctx context.Context) {
	if t.isRunned() {
		t.eServer.log.Warnw("topic already runned",
			"eSrvID", t.eServer.ID,
			"eSrvName", t.eServer.Name,
			"topic", t.fullName)

		return
	}

	t.Lock()
	t.runned = true
	t.ctx = ctx
	t.Unlock()

	t.eServer.log.Debugw("topic execution started...",
		"eSrvID", t.eServer.ID,
		"eSrvName", t.eServer.Name,
		"topic", t.fullName)

	// start all subtopics
	go func() {
		t.Lock()
		defer t.Unlock()

		for _, st := range t.subtopics {
			st.run(ctx)
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				t.Lock()
				t.runned = false
				t.Unlock()

				t.eServer.log.Debugw("topic execution stopped",
					"eSrvID", t.eServer.ID,
					"eSrvName", t.eServer.Name,
					"topic", t.fullName)

				return

			// register the event
			case ee := <-t.inCh:
				if ee.event == nil {
					t.eServer.log.Warnw("got empty event in envelope",
						"eSrvID", t.eServer.ID,
						"eSrvName", t.eServer.Name,
						"topic", t.fullName)

					continue
				}

				if ee.Publisher == uuid.Nil {
					t.eServer.log.Warnw("event has no publisher id",
						"eSrvID", t.eServer.ID,
						"eSrvName", t.eServer.Name,
						"topic", t.fullName,
						"event", ee.event.String())

					continue
				}

				ee.RegAt = time.Now()

				t.Lock()
				pos := len(t.events)
				ee.Index = pos
				t.events = append(t.events, ee)
				t.Unlock()

				t.eServer.log.Debugw("new event registered",
					"eSrvID", t.eServer.ID,
					"eSrvName", t.eServer.Name,
					"topic", t.fullName,
					"evtName", ee.event.Name)

				// send event for subscribers
				go t.updateSubs(ctx, &ee, pos)
			}
		}
	}()
}

// updateSubs sends all the subscribers a single EventEnvelope.
func (t *Topic) updateSubs(ctx context.Context, ee *EventEnvelope, pos int) {
	t.Lock()
	defer t.Unlock()

	for _, s := range t.subs {

		s := s

		go func() {
			// check if there is a filter and event comply its conditions.
			if s.filterCond != nil {
				if ee = s.filter(ee); ee != nil {
					// try to send event or stop on context's cancel
					select {
					case s.eCh <- *ee:
					case <-ctx.Done():
					}
				}

				return
			}

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
		}()
	}
}

// -----------------------------------------------------------------------------
//                    Service functions

// update2Absolute makes the path absolute by adding
// '/' at the begin of the path.
func update2Absolute(path string) string {
	path = strings.Trim(path, " ")

	if len(path) == 0 {
		return "/"
	}

	if path[0] != '/' {
		path = "/" + path
	}

	return path
}

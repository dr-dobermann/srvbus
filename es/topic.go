package es

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

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
	subs map[uuid.UUID][]*subscription

	// running flag
	runned bool

	// runned context
	ctx context.Context

	// topic logger
	log zap.SugaredLogger
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
			subs:      map[uuid.UUID][]*subscription{}}
		t.subtopics[name] = nt

		if t.runned {
			nt.run(t.ctx)
		}

		t.log.Debugw("subtopic added",
			"subtopic", name)

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
		t.log.Warn("topic already runned")

		return
	}

	t.Lock()
	t.runned = true
	t.ctx = ctx
	t.log = *t.eServer.log.Named(t.fullName)
	t.Unlock()

	t.log.Debug("topic execution started...")

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

				t.log.Debug("topic execution stopped")

				return

			// register the event
			case ee := <-t.inCh:
				if err := ee.check(); err != nil {
					t.log.Warnw("invalid envelope",
						"err", err.Error())

					continue
				}

				ee.RegAt = time.Now()

				t.Lock()
				pos := len(t.events)
				ee.Index = pos
				t.events = append(t.events, ee)
				t.Unlock()

				t.log.Debugw("new event registered",
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

	for _, sl := range t.subs {

		ss := sl

		go func() {
			// go throug one subscriber subsciptions
			for _, s := range ss {
				s := s
				go s.sendEvent(ctx, ee, pos)
			}
		}()
	}
}

// creating single subscritpition from the subscription request sr.
func (t *Topic) subscribe(subscriber uuid.UUID, sr *SubscrReq) error {

	return errNotImplementedYet
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

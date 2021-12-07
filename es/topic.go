package es

import (
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type EventEnvelope struct {
	event *Event

	publisher uuid.UUID
	regAt     time.Time
}

// subscription keeps status for one Subscriber.
type subscription struct {
	// subscriber id
	id uuid.UUID

	// subscription request
	subReq string

	// channel for subscribed topics
	eCh chan EventEnvelope

	// last readed index
	lastReaded map[string]int
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

	// subscribers for the queue
	subs map[uuid.UUID]subscription
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

		t.subtopics[name] = &Topic{
			eServer:   t.eServer,
			fullName:  t.fullName + "/" + name,
			name:      name,
			events:    []EventEnvelope{},
			subtopics: map[string]*Topic{},
			subs:      map[uuid.UUID]subscription{}}

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

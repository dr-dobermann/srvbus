// srvBus is a Service Providing Server developed to
// support project GoBPM.
//
// (c) 2021, Ruslan Gabitov a.k.a. dr-dobermann.
// Use of this source is governed by LGPL license that
// can be found in the LICENSE file.
//
/*
Package es is a part of the srvbus package. es consists of the
in-memory Events Server implementation.

Event Server provides the sub/pub model of the data exchange.
*/
package es

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type EventServerError struct {
	ID   uuid.UUID
	Name string
	Msg  string
	Err  error
}

// newESErr creates a new EventServiceError object.
func newESErr(
	eSrv *EventServer,
	err error,
	format string,
	params ...interface{}) EventServerError {

	return EventServerError{ID: eSrv.ID,
		Name: eSrv.Name,
		Msg:  fmt.Sprintf(format, params...),
		Err:  err}
}

// Error implements fmt.Error interface for EventServiceError
func (ese EventServerError) Error() string {

	return fmt.Sprintf("ES '%s' # %v ERR: %s : %v", ese.Name,
		ese.ID,
		ese.Msg,
		ese.Err)
}

// EventServer keeps the state of the event server.
type EventServer struct {
	sync.Mutex

	ID   uuid.UUID
	Name string

	log *zap.SugaredLogger

	topics map[string]*Topic

	// running flag
	runned bool

	// running context
	ctx context.Context
}

// Logger returns a pointer to the internal logger of
// the EventServer
func (eSrv *EventServer) Logger() *zap.SugaredLogger {
	return eSrv.log
}

// IsRunned returns current running state of the EventServer
func (eSrv *EventServer) IsRunned() bool {
	eSrv.Lock()
	defer eSrv.Unlock()

	return eSrv.runned
}

// HasTopic checks if the topic exists on the EventServer.
//
// If topic is existed, then true returned, false oterwise.
func (eSrv *EventServer) HasTopic(name string) bool {
	if _, has := eSrv.hasTopic(name); has {
		return true
	}

	return false
}

// hasTopic returns true if the topic is presented on the EventServer.
//
// In addition to existance flag hasTopic also returns the pointer to
// the topic.
func (eSrv *EventServer) hasTopic(name string) (*Topic, bool) {
	// parse topic
	tt := []string{}
	for _, t := range strings.Split(name, "/") {
		t = strings.Trim(t, " ")
		if len(t) != 0 {
			tt = append(tt, t)
		}
	}

	// return false if there is no valid topic names
	if len(tt) == 0 {
		return nil, false
	}

	eSrv.Lock()
	defer eSrv.Unlock()

	// if the first topic isn't exist on the eSrv return false
	t, ok := eSrv.topics[update2Absolute(tt[0])]
	if !ok {
		return nil, false
	}

	// if there are subtopics, call Topic.hasTopic and return
	// its result
	if len(tt) > 1 {
		return t.hasSubtopic(tt[1:])
	}

	return t, true
}

// AddTopic adds a new topic `name` into the EventServer topic tree.
//
// branch consist of list of topics which are over the new one.
// It looks like "topic\subtopic\subsubtopic". If the new topic
// should be on the root of the EventServer, then its branch == "/".
// There is only absolute topic's path if the first letter of the branch
// isn't '/' then it assumed as the first topic from the root
// "topic" == "/topic".
//
// Topics could be added even when the EventServer is not running,
// but if it runs with cleanStart == true !!!ALL TOPICS WILL BE LOST!!!
//
func (eSrv *EventServer) AddTopic(name string, branch string) error {
	name = strings.Trim(name, " ")
	if name == "" {
		return newESErr(eSrv, nil, "empty topic name is not allowed")
	}

	// parse branch
	base := []string{}
	for _, t := range strings.Split(branch, "/") {
		t = strings.Trim(t, " ")
		if len(t) != 0 {
			base = append(base, t)
		}
	}

	eSrv.Lock()
	defer eSrv.Unlock()

	// if baseTopis is root, add it to eSrv.topics
	if len(base) == 0 {
		name = update2Absolute(name)
		// check for duplicates on eSrv
		if _, ok := eSrv.topics[name]; ok {
			return newESErr(eSrv, nil, "topic '%s' already exists", name)
		}

		nt := &Topic{
			eServer:   eSrv,
			fullName:  name,
			name:      name,
			events:    []EventEnvelope{},
			subtopics: map[string]*Topic{},
			inCh:      make(chan EventEnvelope),
			subs:      map[uuid.UUID]subscription{}}

		eSrv.topics[name] = nt

		eSrv.log.Debugw("topic added to root",
			"eSrvID", eSrv.ID,
			"eSrvName", eSrv.Name,
			"topic", name)

		if eSrv.runned {
			nt.run(eSrv.ctx)
		}

		return nil
	}

	// if there is topic in eSrv.topics which is the first
	// topic in branch then call it addSubtopic method
	// for it and send to it all branchs slices except the first one.
	base[0] = update2Absolute(base[0])
	t, ok := eSrv.topics[base[0]]
	if !ok {
		return newESErr(eSrv, nil, "no '%s' topic on server", base[0])
	}

	return t.addSubtopic(name, base[1:])
}

// AddTopicQueue add a whole branch of topics at once.
func (eSrv *EventServer) AddTopicQueue(
	topicsQueue string,
	branch string) error {

	// check if there is base topic on the server
	branch = strings.Trim(branch, " ")
	if branch != "" && branch != "/" && !eSrv.HasTopic(branch) {
		return newESErr(eSrv, nil,
			"no topic '%s'", branch)
	}

	// parse the topicQueue
	for _, t := range strings.Split(topicsQueue, "/") {
		t = strings.Trim(t, " ")

		// add every non-empty topic and update the
		// branch with it for the next one.
		if len(t) > 0 {
			if err := eSrv.AddTopic(t, branch); err != nil {
				return newESErr(eSrv, err,
					"couldn't add topic '%s' to '%s'", t, branch)
			}
			branch += "/" + t
		}
	}

	return nil
}

// AddEvent add an Event into the topic.
//
// AddEvent doesn't support the event sequental order.
func (eSrv *EventServer) AddEvent(
	topic string,
	evt *Event,
	sender uuid.UUID) error {

	if !eSrv.IsRunned() {
		return newESErr(eSrv, nil, "couldn't add event on not-runned server")
	}

	if evt == nil {
		return newESErr(eSrv, nil, "empty event registration")
	}

	if sender == uuid.Nil {
		return newESErr(eSrv, nil, "no sender for Event '%s'", evt.Name)
	}

	t, found := eSrv.hasTopic(topic)

	if !found {
		return newESErr(eSrv, nil, "no topic '%s' on server", topic)
	}

	go func() {
		ee := &EventEnvelope{
			event:     evt,
			topic:     topic,
			publisher: sender}

		select {
		case <-eSrv.ctx.Done():
			return

		case t.inCh <- *ee:
		}
	}()

	return nil
}

const (
	default_topic = "/server"
)

// Creates a new EventServer.
func New(
	id uuid.UUID,
	name string,
	log *zap.SugaredLogger) (*EventServer, error) {

	if id == uuid.Nil {
		id = uuid.New()
	}

	if name == "" {
		name = "EventServer #" + id.String()
	}
	if log == nil {
		return nil,
			fmt.Errorf("log is absent for serverv %s # %v",
				name, id)
	}

	eSrv := new(EventServer)
	eSrv.Name = name
	eSrv.ID = id
	eSrv.log = log
	eSrv.topics = make(map[string]*Topic)

	eSrv.log.Infow("event server created",
		"eSrvID", eSrv.ID,
		"eSrvName", eSrv.Name)

	// add server's default topic
	if err := eSrv.AddTopic(default_topic, "/"); err != nil {
		return nil,
			newESErr(
				eSrv,
				err,
				"couldn't add default topic '%s'", default_topic)
	}

	return eSrv, nil
}

// Run starts the EventServer.
//
// To stope server use context's cancel function.
func (eSrv *EventServer) Run(ctx context.Context, cleanStart bool) error {
	if eSrv.IsRunned() {
		return newESErr(eSrv, nil, "server already started")
	}

	eSrv.log.Infow("event server is starting...",
		"eSrvID", eSrv.ID,
		"eSrvName", eSrv.Name,
		"cleanStart", cleanStart)

	// create new topics table or clean it if needed
	if cleanStart {
		eSrv.topics = make(map[string]*Topic)

		// add server's default topic
		if err := eSrv.AddTopic(default_topic, "/"); err != nil {
			return newESErr(
				eSrv,
				err,
				"couldn't add default topic '%s'", default_topic)
		}
	}

	go func() {
		<-ctx.Done()
		eSrv.Lock()
		eSrv.runned = false
		eSrv.Unlock()

		eSrv.log.Infow("event server stopped",
			"eSrvID", eSrv.ID,
			"eSrvName", eSrv.Name,
			"err", ctx.Err())
	}()

	eSrv.ctx = ctx
	eSrv.runned = true

	// run all topics
	for _, t := range eSrv.topics {
		t.run(ctx)
	}

	eSrv.log.Infow("event server started",
		"eSrvID", eSrv.ID,
		"eSrvName", eSrv.Name)

	return nil
}

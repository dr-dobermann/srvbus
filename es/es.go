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

It could be used separately from the others srvbus pacackages.
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

const (
	topicNew = "NEW_TOPIC_EVT"
	topicDel = "TOPIC_REMOVE_EVT"

	subsNew = "NEW_SUBSCRIPTION_EVT"
	subsDel = "SUBSCRIPTION_REMOVE_EVT"

	defaultTopic = "/server"

	RootTopic = "/"

	// Run flag
	NormalRun = false
	CleanRun  = true
)

// EventServerError is a error wrapper for Event Server and his things.
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

	return fmt.Sprintf("ES: [%s] # %v. ERROR: %s : %v",
		ese.Name,
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

	esTopic string
}

// emits single event into the personal message server topic
// if the Event Server was given on New call.
func (eSrv *EventServer) emitEvent(name, descr string) {

	// initialize default server topic if needed
	if eSrv.esTopic == "" {
		topic := defaultTopic
		if err := eSrv.AddTopicQueue(topic, "/"); err != nil {
			eSrv.log.Warnw("couldn't add topic to Event Server",
				zap.String("topic", topic),
				zap.Error(err))

			return
		}
		eSrv.esTopic = topic
	}

	EmitEvent(eSrv, eSrv.esTopic, name, descr, eSrv.ID)
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
// Topics couldn't be added when the EventServer is not running,
// if event server reruns with cleanStart == true
// '!!!ALL TOPICS WILL BE LOST!!!
//
func (eSrv *EventServer) AddTopic(name string, branch string) error {
	if !eSrv.IsRunned() {
		return newESErr(eSrv, nil, "couldn't add topic on not-runned server")
	}

	name = strings.Trim(name, " ")
	if name == "" {
		return newESErr(eSrv, nil, "empty topic name is not allowed")
	}

	// normalize name. It should only consists the topic name.
	// if there are leading topics in name they should be movet to the
	// end of the branch.
	//
	// So name="/topic/subtopic", branch="/main" should become
	// name="subtopic", branch=/main/topic
	nn := strings.Split(name, "/")
	var pos int
	for pos = len(nn) - 1; pos > 0; pos-- {
		if len(nn[pos]) > 0 {
			name = nn[pos]
			break
		}
	}

	if pos < 0 {
		return newESErr(eSrv, nil, "invalid topic name '%s'", name)
	}

	for i := 0; i < pos; i++ {
		if len(nn[i]) > 0 {
			branch += "/" + nn[i]
		}
	}

	// parse branch
	base := []string{}
	for _, t := range strings.Split(branch, "/") {
		t = strings.Trim(t, " ")
		if len(t) != 0 {
			base = append(base, t)
		}
	}

	// if baseTopis is root, add it to eSrv.topics
	if len(base) == 0 {
		name = update2Absolute(name)
		// check for duplicates on eSrv
		eSrv.Lock()
		_, ok := eSrv.topics[name]
		eSrv.Unlock()

		if ok {
			return newESErr(eSrv, nil, "topic '%s' already exists", name)
		}

		nt := &Topic{
			eServer:   eSrv,
			fullName:  name,
			name:      name,
			events:    []EventEnvelope{},
			subtopics: map[string]*Topic{},
			inCh:      make(chan EventEnvelope),
			log:       *eSrv.Logger().Named(name),
			subs:      map[uuid.UUID][]*subscription{}}

		eSrv.Lock()
		eSrv.topics[name] = nt
		eSrv.Unlock()

		eSrv.log.Debugw("topic added to root",
			"topic", name)

		// if server is runned, run the topic too
		if eSrv.IsRunned() {
			nt.run(eSrv.ctx)
		}

		eSrv.emitEvent(topicNew, "{topic: "+name+"}")

		return nil
	}

	// if there is topic in eSrv.topics which is the first
	// topic in branch then call it addSubtopic method
	// for it and send to it all branchs slices except the first one.
	base[0] = update2Absolute(base[0])
	eSrv.Lock()
	t, ok := eSrv.topics[base[0]]
	eSrv.Unlock()

	if !ok {
		return newESErr(eSrv, nil, "no '%s' topic on server", base[0])
	}

	err := t.addSubtopic(name, base[1:])

	if err != nil {
		return newESErr(eSrv, err,
			"couldn't add subtopic '%s' topic to '%s'", name, branch)
	}

	eSrv.emitEvent(topicNew, "{topic: "+name+"}")

	return nil
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

// RemoveTopic removes single topic or topics' subbranch if recursive == true.
func (eSrv *EventServer) RemoveTopic(topic string, recursive bool) error {
	if !eSrv.IsRunned() {
		return newESErr(eSrv, nil, "couldn't remove topic on stopped server")
	}

	t, found := eSrv.hasTopic(topic)
	if !found {
		return newESErr(eSrv, nil, "topic isn't found")
	}

	// if it's a root topic
	tt := strings.Split(t.fullName, "/")[1:]
	if len(tt) == 1 {
		// delete topic's subtopics if it's recursive
		if err := t.removeSubtopics("", recursive); err != nil {
			return newESErr(
				eSrv, err,
				"couldn't remove subtopics of '%s'", topic)
		}

		// stop topic
		t.cancelCtx()

		// delete topic
		eSrv.Lock()
		delete(eSrv.topics, topic)
		eSrv.Unlock()

		eSrv.log.Debugw("root topic deleted",
			"topic", topic)

		eSrv.emitEvent(topicDel, topic)

		return nil
	}

	// get the topic which owns the selected one
	tn := ""
	for _, s := range tt[:len(tt)-1] {
		tn += "/" + s
	}

	// subtopic name that should be deleted
	dt := tt[len(tt)-1]

	t, found = eSrv.hasTopic(tn)
	if !found {
		return newESErr(eSrv, nil, "couldn't find topic '%s'", tn)
	}

	if err := t.removeSubtopics(dt, recursive); err != nil {
		return newESErr(eSrv, err, "couldn't remove topic '%s'", topic)
	}

	eSrv.log.Debugw("topic deleted",
		"topic", topic)

	eSrv.emitEvent(topicDel, "{topic: "+topic+"}")

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

	ee := EventEnvelope{
		event:     evt,
		Topic:     topic,
		Publisher: sender}

	go func() {
		select {
		case <-eSrv.ctx.Done():
			return

		case t.inCh <- ee:
		}
	}()

	return nil
}

// Subscribe creates an one or many one subscriber's subscriptions.
func (eSrv *EventServer) Subscribe(
	subscriber uuid.UUID,
	subs ...SubscrReq) error {

	if !eSrv.IsRunned() {
		return newESErr(eSrv, nil, "couldn't subscribe on a not-runned server")
	}

	if subscriber == uuid.Nil {
		return newESErr(eSrv, nil, "no subscriber given")
	}

	for i, s := range subs {
		if err := s.check(); err != nil {
			return newESErr(eSrv, err, "bad subscription request #%d", i)
		}

		t, found := eSrv.hasTopic(s.Topic)
		if !found {
			return newESErr(
				eSrv,
				nil,
				"couldn't subscribe to non-existed topic '%s'", s.Topic)
		}

		if err := t.subscribe(subscriber, s); err != nil {
			return newESErr(eSrv, err, "subscription #%d failed", i)
		}

		eSrv.emitEvent(subsNew,
			fmt.Sprintf("{subscriber_id: \"%v\", topic: \"%s\"}",
				subscriber, s.Topic))
	}

	return nil
}

// UnSubscribe cancels one or many subscriptions of one subscriber.
func (eSrv *EventServer) UnSubscribe(
	subscriber uuid.UUID,
	topics ...string) error {

	if !eSrv.IsRunned() {
		return newESErr(eSrv, nil, "couldn't unsubscribe on stopped server")
	}

	if subscriber == uuid.Nil {
		return newESErr(eSrv, nil, "no subscriber given")
	}

	for _, s := range topics {
		t, found := eSrv.hasTopic(s)
		if !found {
			return newESErr(
				eSrv,
				nil,
				"couldn't unsubscribe from non-existed topic %s", s)
		}

		if err := t.unsubscribe(subscriber); err != nil {
			return newESErr(
				eSrv,
				err,
				"unsubscription form topic %s failed", s)
		}

		eSrv.emitEvent(subsDel,
			fmt.Sprintf("{subscriber_id: \"%v\", topic: \"%s\"",
				subscriber, s))
	}

	return nil
}

// Creates a new EventServer.
func New(
	id uuid.UUID,
	name string,
	log *zap.SugaredLogger) (*EventServer, error) {

	if id == uuid.Nil {
		id = uuid.New()
	}

	if name == "" {
		i := id.String()
		name = "UNNAMED #" + i[len(i)-4:]
	}

	if log == nil {
		return nil,
			fmt.Errorf("log is absent for serverv %s # %v",
				name, id)
	}

	eSrv := new(EventServer)
	eSrv.Name = name
	eSrv.ID = id
	eSrv.log = log.Named("ES: [" + eSrv.Name +
		"] #" + eSrv.ID.String())
	eSrv.topics = make(map[string]*Topic)

	eSrv.log.Info("event server created")

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
		"cleanStart", cleanStart)

	// create new topics table or clean it if needed
	if cleanStart {
		eSrv.topics = make(map[string]*Topic)
	}

	go func() {
		<-ctx.Done()
		eSrv.Lock()
		eSrv.runned = false
		eSrv.Unlock()

		eSrv.log.Infow("event server stopped",
			"err", ctx.Err())

	}()

	eSrv.ctx = ctx
	eSrv.runned = true

	// add server's default topic
	if err := eSrv.AddTopic(defaultTopic, "/"); err != nil {
		return newESErr(
			eSrv,
			err,
			"couldn't add default topic '%s'", defaultTopic)
	}

	eSrv.log.Info("event server started")

	return nil
}

// =============================================================================
//                              Statistics
//
type TopicInfo struct {
	Name        string
	FullName    string
	Runned      bool
	Events      int
	Subtopics   []TopicInfo
	Subscribers []uuid.UUID
}

// Statistics returns Event Server statistics
func (eSrv *EventServer) Statistics() []TopicInfo {
	res := []TopicInfo{}
	eSrv.Lock()
	defer eSrv.Unlock()

	for _, t := range eSrv.topics {
		res = append(res,
			TopicInfo{
				Name:        t.name,
				FullName:    t.fullName,
				Runned:      t.isRunned(),
				Events:      t.count(),
				Subtopics:   t.getSubtopics(),
				Subscribers: t.getSubscribers()})
	}

	return res
}

// String implements fmt.Stringer for TopicInfo
func (ti TopicInfo) String() string {

	return str(ti, 0)
}

// =============================================================================
// emits single event into the selected topic.
func EmitEvent(
	eSrv *EventServer,
	topic, name, descr string,
	senderID uuid.UUID) {

	if eSrv == nil {
		return
	}

	evt, err := NewEventWithString(name, descr)
	if err != nil {
		eSrv.log.Warnw("couldn't create an event",
			zap.String("evt_name", name),
			zap.String("senderID", senderID.String()),
			zap.Error(err))

		return
	}

	if err := eSrv.AddEvent(topic, evt, senderID); err != nil {
		eSrv.log.Warnw("couldn't register an event",
			zap.String("evt_name", name),
			zap.String("senderID", senderID.String()),
			zap.Error(err))

		return
	}
}

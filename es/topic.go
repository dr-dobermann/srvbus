package es

import (
	"context"
	"fmt"
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

	// context cancel function which could stop
	// topic operations
	cancelCtx context.CancelFunc

	// topic logger
	log zap.SugaredLogger
}

// isRunned returns the running status of the topic
func (t *Topic) isRunned() bool {
	t.Lock()
	defer t.Unlock()

	return t.runned
}

// return envents count in the topic
func (t *Topic) count() int {
	t.Lock()
	defer t.Unlock()

	return len(t.events)
}

// returns subtopics statistics list
func (t *Topic) getSubtopics() []TopicInfo {
	res := []TopicInfo{}

	t.Lock()
	defer t.Unlock()

	for _, st := range t.subtopics {
		res = append(res, TopicInfo{
			Name:        st.name,
			FullName:    st.fullName,
			Runned:      st.isRunned(),
			Events:      st.count(),
			Subtopics:   st.getSubtopics(),
			Subscribers: st.getSubscribers(),
		})
	}

	return res
}

// returns list of subscribers for the topic
func (t *Topic) getSubscribers() []uuid.UUID {
	res := []uuid.UUID{}

	t.Lock()
	defer t.Unlock()

	for sb := range t.subs {
		res = append(res, sb)
	}

	return res
}

// addSubtopic adds a new subtopic if there is no duplicates.
//
// Function gets a new subtopic name and slice of the base topics.
// This slice doesnt consist of t.Name only the topics which are under
// the t.
func (t *Topic) addSubtopic(name string, base []string) error {
	// if current Topic is the last one in the base,
	// add new topic to it and return
	if len(base) == 0 {
		t.Lock()
		_, ok := t.subtopics[name]
		t.Unlock()
		if ok {
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
			log:       *t.log.Named(name),
			subs:      map[uuid.UUID][]*subscription{}}

		t.Lock()

		t.subtopics[name] = nt

		runned := t.runned

		t.Unlock()

		if runned {
			nt.run(t.ctx)
		}

		t.log.Debugw("subtopic added",
			zap.String("subtopic", name))

		return nil
	}

	// check if t has subtopic named as the first out of base.
	// if it exists, add new topic as its subtopic.
	t.Lock()
	st, ok := t.subtopics[base[0]]
	t.Unlock()

	if !ok {
		return newESErr(t.eServer, nil,
			"topic '%s' has no subtopic '%s'", t.fullName, base[0])
	}

	return st.addSubtopic(name, base[1:])
}

// removes one or all subtopics if t doesnt' have branches
// or remove subtopics recursively.
//
// if topic isn't empty string, then removing only one subtopic
// and all its subtopics if recursive.
func (t *Topic) removeSubtopics(topic string, recursive bool) error {
	t.Lock()
	if !recursive &&
		((topic != "" && len(t.subtopics[topic].subtopics) > 0) ||
			(topic == "" && !t.couldBeDeleted())) {
		t.Unlock()

		return newESErr(t.eServer, nil,
			"couldn't remove topic with subtopics tree")
	}
	t.Unlock()

	// remove one or all subtopics
	for _, st := range t.subtopics {
		if topic != "" && st.name != topic {
			continue
		}

		if err := st.removeSubtopics("", recursive); err != nil {
			return newESErr(t.eServer, err,
				"couldn't remove subtopic '%s'", st.fullName)
		}

		// stop subtopic
		st.cancelCtx()

		t.Lock()
		delete(t.subtopics, st.name)
		t.Unlock()

		t.log.Debugw("subtopic deleted",
			zap.String("topic", st.name))
	}

	return nil
}

// checks if t could be deleted without recursion becouse
// t doesn't have branches. It don't have subtopics or
// has just subtopics without subtopics.
func (t *Topic) couldBeDeleted() bool {
	for _, st := range t.subtopics {
		if len(st.subtopics) > 0 {
			return false
		}
	}

	return true
}

// hasSubtopic checks if topics are existed in the topic.
//
// if the given topics has subtopics, they would be checked
// over recursive calls of its hasSubtopic.
func (t *Topic) hasSubtopic(topics []string) (*Topic, bool) {
	// check if t owns the first topic in the topics
	t.Lock()
	st, ok := t.subtopics[topics[0]]
	t.Unlock()

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
	t.ctx, t.cancelCtx = context.WithCancel(ctx)
	t.Unlock()

	t.log.Debug("topic execution started...")

	// register the event
	// send event for subscribers
	go t.processTopic(t.ctx)
}

// processing main topic cycle
func (t *Topic) processTopic(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			t.Lock()
			t.runned = false
			t.Unlock()

			t.log.Debug("topic execution stopped")

			return

		case ee := <-t.inCh:
			if err := ee.check(); err != nil {
				t.log.Warnw("invalid envelope",
					zap.Error(err))

				continue
			}

			ee.RegAt = time.Now()

			t.Lock()
			pos := len(t.events)
			ee.Index = pos
			t.events = append(t.events, ee)
			t.Unlock()

			t.log.Debugw("new event registered",
				zap.String("evtName", ee.event.Name))

			go t.updateSubs(ctx, &ee, pos)

		default:
			if !t.isRunned() {
				return
			}
		}
	}
}

// updateSubs sends all the subscribers a single EventEnvelope.
func (t *Topic) updateSubs(ctx context.Context, ee *EventEnvelope, pos int) {
	t.Lock()
	defer t.Unlock()

	for _, sl := range t.subs {

		ss := sl

		go func() {
			// go through one subscriber subsciptions
			for _, s := range ss {
				s := s
				go s.sendEvent(ctx, ee, pos)
			}
		}()
	}
}

// creating single subscritpition from the subscription request sr.
func (t *Topic) subscribe(subscriber uuid.UUID, sr SubscrReq) error {
	if !t.isRunned() {
		return newESErr(t.eServer, nil, "cannot subscribe on stopped topic")
	}

	s, ok := t.subs[subscriber]
	if !ok {
		s = []*subscription{}
	}

	ns := &subscription{
		subscriber: subscriber,
		subReq:     sr.Topic,
		eCh:        sr.SubCh,
		lastReaded: sr.StartPos,
		filters:    sr.Filters}

	s = append(s, ns)

	t.subs[subscriber] = s

	t.log.Debugw("subscription added",
		zap.Stringer("subscriber", subscriber),
		zap.String("sub_request", sr.Topic),
		zap.Bool("recursive", sr.Recursive),
		zap.Uint("rec_depth", sr.Depth))

	go t.startSub(ns, sr.StartPos)

	// if the subscription is recursive, add subscription to subtopics
	if sr.Recursive && sr.Depth > 0 {
		sr.Depth = sr.Depth - 1
		for _, st := range t.subtopics {
			if err := st.subscribe(subscriber, sr); err != nil {
				return newESErr(
					t.eServer,
					err,
					"couldn't subscribe a subtopic '%s'",
					st.fullName)
			}
		}
	}

	return nil
}

// unsubscribes subscriber from the topic.
func (t *Topic) unsubscribe(subscriber uuid.UUID) error {
	if !t.isRunned() {
		return newESErr(
			t.eServer,
			nil,
			"cannot unsubscribe from a stopped topic")
	}

	t.Lock()
	defer t.Unlock()

	if _, ok := t.subs[subscriber]; !ok {
		return newESErr(
			t.eServer,
			nil,
			"subscriber #%v has no subscriptions on topic %s",
			subscriber, t.fullName)
	}

	delete(t.subs, subscriber)

	t.log.Debugw("subscription cancelled",
		zap.Stringer("subscriber", subscriber))

	return nil
}

// startSub sends all previous events to the newly created subscription
// if it asks for them.
func (t *Topic) startSub(s *subscription, fromID int) {
	t.Lock()
	defer t.Unlock()

	if fromID < 0 {
		s.Lock()
		s.lastReaded = len(t.events) - 1
		s.Unlock()
		return
	}

	for pos, ee := range t.events[fromID:] {
		ee := ee
		go s.sendEvent(t.ctx, &ee, pos)
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

// converts ti into string.
func str(ti TopicInfo, offset int) string {
	offs := strings.Repeat(" ", offset*2)

	res := offs + ti.FullName

	if ti.Runned {
		res += " [Runned]"
	} else {
		res += " [Not runned]"
	}

	res += fmt.Sprintf(" events(%d)\n%s  subscribers:\n", ti.Events, offs)

	for _, sb := range ti.Subscribers {
		res += offs + "    " + sb.String() + "\n"
	}

	res += offs + "  subtopics:\n"

	for _, st := range ti.Subtopics {
		res += offs + str(st, offset+1) + "\n"
	}

	return res
}

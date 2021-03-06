// srvBus is a Service Providing Server developed to
// support project GoBPM.
//
// (c) 2021, Ruslan Gabitov a.k.a. dr-dobermann.
// Use of this source is governed by LGPL license that
// can be found in the LICENSE file.

/*
Package ms is a part of srvbus package. It consists of the
Message Server implementation.

Message Server provides simple in-memory queued messages interchange
server.

It could be used separately of the rest of the srvbus packages.
*/
package ms

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dr-dobermann/srvbus/es"
	"github.com/dr-dobermann/srvbus/internal/errs"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	srvNew   = "NEW_MSERVER_EVT"
	srvStart = "MSERVER_START_EVT"
	srvEnd   = "MSERVER_END_EVT"

	defaultTopicName = "/mserver"
)

/// MessageServer holds all the Message Server data.
type MessageServer struct {
	sync.Mutex

	id   uuid.UUID
	Name string
	log  *zap.SugaredLogger

	queues map[string]*mQueue

	ctx context.Context

	runned bool

	eSrv    *es.EventServer
	esTopic string
}

// emits single event into the personal message server topic
// if the Event Server was given on New call.
func (mSrv *MessageServer) EmitEvent(name, descr string) {
	if mSrv.eSrv == nil || !mSrv.eSrv.IsRunned() {
		mSrv.log.Warnw("couldn't register event on non-runned or "+
			"empty event server",
			zap.String("name", name))

		return
	}

	if descr == "" {
		descr = fmt.Sprintf(
			"{name: \"%s\", id: \"%s\"}",
			mSrv.Name, mSrv.id)
	}

	// initialize default server topic if needed
	if mSrv.esTopic == "" {
		topic := defaultTopicName + "/" + mSrv.id.String()
		if err := mSrv.eSrv.AddTopicQueue(topic, es.RootTopic); err != nil {
			mSrv.log.Warnw("couldn't add topic to Event Server",
				zap.String("eSrvName", mSrv.eSrv.Name),
				zap.Stringer("eSrvID", mSrv.eSrv.ID),
				zap.String("topic", topic),
				zap.Error(err))

			return
		}
		mSrv.esTopic = topic
	}

	es.EmitEvt(mSrv.eSrv, mSrv.esTopic, name, descr, mSrv.id)
}

// returns ID of the server
func (mSrv *MessageServer) ID() uuid.UUID {
	return mSrv.id
}

// returns event_service topic for the message server
func (mSrv *MessageServer) ESTopic() string {
	return mSrv.esTopic
}

// returns current loggers of the Message Server
func (mSrv *MessageServer) Logger() *zap.SugaredLogger {
	return mSrv.log
}

// IsRunned returns the current running state of the MessageServer.
func (mSrv *MessageServer) IsRunned() bool {
	mSrv.Lock()
	defer mSrv.Unlock()

	return mSrv.runned
}

// QueueStat represent the statistics for a single queue.
type QueueStat struct {
	Name   string
	MCount int
}

// Queues returns a list of queues on the server
func (mSrv *MessageServer) Queues() []QueueStat {
	mSrv.Lock()
	defer mSrv.Unlock()

	ql := []QueueStat{}

	for n, q := range mSrv.queues {
		ql = append(ql, QueueStat{n, q.count()})
	}

	return ql
}

// HasQueue checks if the queue is present on the server.
//
// if the server isn't running, false will be returned.
func (mSrv *MessageServer) HasQueue(queue string) bool {
	if !mSrv.IsRunned() {
		return false
	}

	mSrv.Lock()
	defer mSrv.Unlock()

	_, ok := mSrv.queues[queue]

	return ok
}

// New creates a new Message Server and returns its pointer.
//
// New starts internal go-routine to catch ctx.Done() signal
// and stops the server.
func New(
	id uuid.UUID,
	name string,
	log *zap.SugaredLogger,
	eSrv *es.EventServer) (*MessageServer, error) {

	if log == nil {
		return nil, errs.ErrNoLogger
	}

	if id == uuid.Nil {
		id = uuid.New()
	}

	name = strings.Trim(name, " ")
	if name == "" {
		name = "MsgServer #" + id.String()
	}

	ms := &MessageServer{
		id:   id,
		Name: name,
		log:  log.Named("MS [" + name + "] #" + id.String()),
		eSrv: eSrv}

	log.Debug("message server created")

	ms.EmitEvent(srvNew, "")

	return ms, nil
}

// Run starts the Message Server if it isn't started.
func (mSrv *MessageServer) Run(ctx context.Context) error {
	if mSrv.IsRunned() {
		return errs.ErrAlreadyRunned
	}

	// all old queues should be deleted
	mSrv.Lock()
	mSrv.queues = map[string]*mQueue{}
	mSrv.ctx = ctx
	mSrv.runned = true
	mSrv.Unlock()

	mSrv.log.Info("server started")

	mSrv.EmitEvent(srvStart, "")

	go func() {
		<-ctx.Done()

		mSrv.Lock()

		mSrv.runned = false

		mSrv.Unlock()

		mSrv.log.Info("server stopped")

		mSrv.EmitEvent(srvEnd, "")
	}()

	return nil
}

// PutMessages inserts messages into the queue.
//
// if queue name is empty error will be fired.
//
// if the queue isn't present on the server the new one queue will be
// created and added to the server.
// if some error occurred during the queue creation, the error will be
// returned.
func (mSrv *MessageServer) PutMessages(
	sender uuid.UUID,
	queue string,
	msgs ...*Message) error {

	queue = strings.Trim(queue, " ")
	if queue == "" {
		return errs.ErrEmptyQueueName
	}

	if sender == uuid.Nil {
		return errs.ErrNoSender
	}

	if len(msgs) == 0 {
		return errs.ErrEmptyMessage
	}

	mSrv.Lock()
	defer mSrv.Unlock()

	// check if the server is running
	if !mSrv.runned {
		return errs.ErrNotRunned
	}

	q, ok := mSrv.queues[queue]
	if !ok {
		nq := newQueue(mSrv.ctx, queue, mSrv)

		mSrv.queues[queue] = nq

		q = nq
	}

	return q.putMessages(mSrv.ctx, sender, msgs...)
}

// GetMessages reads all new messages form the queue and returns
// slice of their MessageEnvelopes.
//
// Receiver mustn't be nil and the queue should be on the server of
// error would be returned.
//
// If fromBegin is true, then all messages would be readed from the queue.
func (mSrv *MessageServer) GetMessages(
	receiver uuid.UUID,
	queue string,
	fromBegin bool) (chan MessageEnvelope, error) {

	if receiver == uuid.Nil {
		return nil,
			fmt.Errorf(
				"receiver of message isn't set for queue %s",
				queue)
	}

	// check if the server is ruuning
	if !mSrv.IsRunned() {
		return nil, errs.ErrNotRunned
	}

	mSrv.Lock()
	q, ok := mSrv.queues[queue]
	mSrv.Unlock()

	if !ok {
		return nil,
			fmt.Errorf("queue '%s' isn't found on the server '%s'",
				queue, mSrv.Name)
	}

	return q.getMessages(mSrv.ctx, receiver, fromBegin)
}

// WaitForQueue checks if a queue is present.
//
// if there queue name is empty or if the server isn't runned
// then nil, error will be returned
//
// WaitForQueue returned result channel. The time of waiting is
// controlled outside the WaitForQueue over the context.
//
// If the queue appears while waiting time the channel got true.
// If timeout is exceeded or context cancelled the false will put
// into the channel
//
// For example, if it's needed to wait for queue appears no more
// than 2 seconds, the context should be created as followed:
//    wCtx, wCancel := context.WithDeadline(ctx, time.Now().Add(2 * time.Second))
//	  defer wCancel
//
//    wCh, err := mSrv.WaitForQueue(wCtx, "queue_name")
//    if err != nil {
//       panic(err.Error())
//    }
//
//    res := <-wCh
//    close(wCh)
//
//    if !res {
//       // do compensation
//    }
//
//    // do actual things (i.e. GetMessages)
//
func (mSrv *MessageServer) WaitForQueue(
	ctx context.Context,
	queue string) (chan bool, error) {

	if !mSrv.IsRunned() {
		return nil, errs.ErrNotRunned
	}

	if queue == "" {
		return nil, errs.ErrEmptyQueueName
	}

	wCh := make(chan bool, 1)

	go func() {
		for {
			select {
			case <-ctx.Done():
				wCh <- false

				return

			case <-time.After(time.Second):
				if mSrv.HasQueue(queue) {
					wCh <- true

					return
				}
			}
		}
	}()

	return wCh, nil
}

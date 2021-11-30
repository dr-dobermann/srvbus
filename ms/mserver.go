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
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"
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
func (mSrv *MessageServer) HasQueue(queue string) bool {
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
	log *zap.SugaredLogger) (*MessageServer, error) {
	if log == nil {
		return nil, fmt.Errorf("logger isn't present")
	}

	if id == uuid.Nil {
		id = uuid.New()
	}

	if name == "" {
		name = "MsgServer #" + id.String()
	}

	ms := &MessageServer{
		id:   id,
		Name: name,
		log:  log}

	log.Debugw("message server created",
		"mSrvID", ms.id,
		"name", ms.Name)

	return ms, nil
}

// Run starts the Message Server if it isn't started.
func (mSrv *MessageServer) Run(ctx context.Context) {
	if mSrv.IsRunned() {
		mSrv.log.Infow("alredy runned",
			"mSrvID", mSrv.id,
			"name", mSrv.Name)

		return
	}

	// all old queues should be deleted
	mSrv.Lock()
	mSrv.queues = map[string]*mQueue{}
	mSrv.ctx = ctx
	mSrv.runned = true
	mSrv.Unlock()

	mSrv.log.Infow("message server started",
		"mSrvID", mSrv.id,
		"name", mSrv.Name)

	go func() {
		<-ctx.Done()

		mSrv.Lock()

		mSrv.runned = false

		mSrv.Unlock()

		mSrv.log.Infow("message server stopped",
			"mSrvID", mSrv.id,
			"name", mSrv.Name)
	}()
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
	if queue == "" {
		return fmt.Errorf("empty queue name for messages putting")
	}

	mSrv.Lock()
	defer mSrv.Unlock()

	// check if the server is running
	if !mSrv.runned {
		return fmt.Errorf("server isn't runned")
	}

	_, ok := mSrv.queues[queue]
	if !ok {
		q, err := newQueue(mSrv.ctx, uuid.New(), queue, mSrv.log)
		if err != nil {
			mSrv.log.Errorw("couldn't put messages",
				"queue", queue,
				"err", err.Error())

			return fmt.Errorf("put messages failde : %w", err)
		}

		mSrv.queues[queue] = q
	}

	q := mSrv.queues[queue]

	mSrv.log.Debugw("putting messages", "queue", q.name)

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
	fromBegin bool) ([]MessageEnvelope, error) {
	q, ok := mSrv.queues[queue]
	if !ok {
		mSrv.log.Errorw("no queue", "queue", queue)

		return nil,
			fmt.Errorf("queue '%s' isn't found on the server '%s'",
				queue, mSrv.Name)
	}

	mSrv.Lock()
	runned := mSrv.runned
	mSrv.Unlock()

	// check if the server is ruuning
	if !runned {
		return nil, fmt.Errorf("server isn't runned")
	}

	mSrv.log.Debugw("getting messages", "queue", q.name)

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
// If timeout is exceeded the false will put into the channel
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
		return nil, fmt.Errorf("server isn't running")
	}

	if queue == "" {
		return nil, fmt.Errorf("queue name is empty")
	}

	wCh := make(chan bool, 1)

	go func() {
		for {
			select {
			case <-ctx.Done():
				wCh <- false

				return

			default:
				if mSrv.HasQueue(queue) {
					wCh <- true

					return
				}
			}
		}
	}()

	return wCh, nil

}

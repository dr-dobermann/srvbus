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

	ctxCh chan context.Context
}

// IsRunned returns the current running state of the MessageServer.
func (mSrv *MessageServer) IsRunned() bool {
	mSrv.Lock()
	defer mSrv.Unlock()

	if mSrv.ctxCh == nil {
		return false
	}

	_, ok := <-mSrv.ctxCh

	return ok
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
		id:     id,
		Name:   name,
		log:    log,
		queues: make(map[string]*mQueue)}

	log.Debugw("Message Server created",
		"name", ms.Name,
		"id", ms.id)

	return ms, nil
}

// Run starts the Message Server if it isn't started.
func (mSrv *MessageServer) Run(ctx context.Context) {
	if mSrv.IsRunned() {
		mSrv.log.Infow("alredy runned",
			"id", mSrv.id,
			"name", mSrv.Name)

		return
	}

	// in case of restarting the Message Server
	// all old queues should be deleted
	if mSrv.ctxCh != nil {
		mSrv.Lock()
		mSrv.queues = map[string]*mQueue{}
		mSrv.Unlock()
	}

	mSrv.log.Infow("message server started",
		"id", mSrv.id,
		"name", mSrv.Name)

	mSrv.ctxCh = make(chan context.Context)

	go func() {
		for {

			select {
			case <-ctx.Done():
				mSrv.Lock()
				defer mSrv.Unlock()

				for _, q := range mSrv.queues {
					q.stop()
				}

				close(mSrv.ctxCh)

				mSrv.log.Infow("message server stopped",
					"id", mSrv.id,
					"name", mSrv.Name)

			// put an actual context into the channel for
			// the put and get messages functions and to
			// indicate the Server is running.
			case mSrv.ctxCh <- ctx:
			}
		}
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

	// check if the server is ruuning
	if mSrv.ctxCh == nil {
		return fmt.Errorf("server isn't runned")
	}

	ctx, ok := <-mSrv.ctxCh
	if !ok {
		return fmt.Errorf("server isn't runned")
	}

	mSrv.Lock()
	defer mSrv.Unlock()

	_, ok = mSrv.queues[queue]
	if !ok {
		q, err := newQueue(uuid.New(), queue, mSrv.log)
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

	return q.putMessages(ctx, sender, msgs...)
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

	// check if the server is ruuning
	if mSrv.ctxCh == nil {
		return nil, fmt.Errorf("server isn't runned")
	}

	ctx, ok := <-mSrv.ctxCh
	if !ok {
		return nil, fmt.Errorf("server isn't runned")
	}

	mSrv.log.Debugw("getting messages", "queue", q.name)

	return q.getMessages(ctx, receiver, fromBegin)
}

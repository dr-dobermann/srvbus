package s2

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/dr-dobermann/srvbus/ms"
	"github.com/google/uuid"
)

// MustServiceRunner is a helper which returns ServiceRunner if
// there wasn't any error on its creation.
//
// If there was any error, then panic fires
func MustServiceRunner(s ServiceRunner, err error) ServiceRunner {
	if err != nil {
		panic("error while service creation : " + err.Error())
	}

	return s
}

// new.....Service has common signature
//   func(ctx context.Context,
//        params ...serviceParam) (ServiceRunner, error)
//
// the context used here is different from the one used on service
// execution. The context is only used in case there is long
// preparation procedure needed to create a service.
// Usually it would be ignored by "_ context.Context" declaration.

// =============================================================================
//    Output Service
//
// NewOutputService returns an output ServiceFunc which
// puts all values form vl into io.Writer w.
func NewOutputService(
	_ context.Context,
	w io.Writer,
	vl ...interface{}) (ServiceRunner, error) {
	if w == nil {
		return nil, fmt.Errorf("writer isn't presented for OutputSvc")
	}

	outputSvc := func(_ context.Context) error {
		fmt.Fprint(w, vl...)

		return nil
	}

	return ServiceFunc(outputSvc), nil
}

// =============================================================================
//    Put Messages Service
//
// NewPutMessagesService returns ServiceFunc which puts all messages msgs into
// the queue 'queue' on Message Server mSrv.
func NewPutMessagesService(
	_ context.Context,
	mSrv *ms.MessageServer,
	queue string,
	sender uuid.UUID,
	msgs ...*ms.Message) (ServiceRunner, error) {

	if mSrv == nil || queue == "" ||
		sender == uuid.Nil || len(msgs) == 0 {
		return nil,
			fmt.Errorf("invalid parameter for PutMessage Service : "+
				"mSrv(%p), queue name(%s), no sender(%t), msg num(%d)",
				mSrv, queue, sender == uuid.Nil, len(msgs))
	}

	putMessages := func(_ context.Context) error {

		return mSrv.PutMessages(sender, queue, msgs...)
	}

	return ServiceFunc(putMessages), nil
}

// =============================================================================
//      Get Messages Service
//
// newGetMessagesService creates a ServiceFunc which reads all
// available messaes into channel mesCh.
func NewGetMessagesService(
	_ context.Context,
	mSrv *ms.MessageServer,
	queue string,
	receiver uuid.UUID,
	fromBegin bool,
	waitForQueue bool,
	waitingTime time.Duration,
	minMessagesNumber int,
	mesCh chan ms.MessageEnvelope) (ServiceRunner, error) {
	if mSrv == nil ||
		queue == "" ||
		mesCh == nil ||
		receiver == uuid.Nil {
		return nil,
			fmt.Errorf("invalid parameter for PutMessage Service : "+
				"mSrv(%p), queue name(%s), reciever is empty(%t) "+
				"messages channel is nil(%t)",
				mSrv, queue, receiver == uuid.Nil, mesCh == nil)
	}

	getMsgs := func(ctx context.Context) error {
		if !waitForQueue && !mSrv.HasQueue(queue) {
			return fmt.Errorf("no queue '%s' on the server", queue)
		}

		// wait for the queue if it's not existed yet
		if !mSrv.HasQueue(queue) {
			// set the wait-for-the-queue timeout by the derived context
			wCtx, wCancel := context.WithTimeout(ctx, waitingTime)
			defer wCancel()

			ch, err := mSrv.WaitForQueue(wCtx, queue)
			if err != nil {
				return fmt.Errorf("queue '%s' waiting error : %w",
					queue, err)
			}

			res := <-ch
			close(ch)

			if !res {
				return fmt.Errorf("queue '%s' readiness timeout reached",
					queue)
			}
		}

		if minMessagesNumber <= 0 {
			minMessagesNumber = -1
		}

		// reading messages
		mm := []ms.MessageEnvelope{}
		for len(mm) >= minMessagesNumber {
			// check context closing
			select {
			case <-ctx.Done():
				return fmt.Errorf("GetMessages service "+
					"terminated by context : %w", ctx.Err())

			default:
			}

			mes, err := mSrv.GetMessages(receiver, queue, fromBegin)
			if err != nil {
				return fmt.Errorf("GetMessageSvc reading queue "+
					"'%s' messages error : %w",
					queue, err)
			}

			mm = append(mm, mes...)

			// if we should return as many messages as
			// we could get for a single ms.GetMessages call
			// return only currently readed.
			if minMessagesNumber == -1 {
				break
			}
		}

		// send messages into the results channel
		go func() {
			for len(mm) > 0 {
				select {
				// check context closing
				case <-ctx.Done():
					close(mesCh)
					return

				// sent messages one by one
				default:
					m := mm[0]
					mm = append(mm[:0], mm[1:]...)
					mesCh <- m
				}
			}

			close(mesCh)
		}()

		return nil
	}

	return ServiceFunc(getMsgs), nil
}

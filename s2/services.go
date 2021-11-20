package s2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/dr-dobermann/srvbus/msgsrv"
	"github.com/google/uuid"
)

// OutputService simpy prints all its values on console.
type OutputSvc struct {
	svcBase

	w io.Writer
	v []interface{}
}

var ErrOutpSvcNoWriter = errors.New("nil writer given")

var ErrOutpSvcEmptyOut = errors.New("output list is empty")

var ErrOutpSvcFailOutput = errors.New("output error")

// NewOutputService creates a new Service based on OutputSrv.
//
// If Writer w is nil or vv lenght is zero, then error returned.
func NewOutputSvc(
	sname string,
	w io.Writer,
	vv ...interface{}) (Service, error) {

	if w == nil {
		return nil, ErrOutpSvcNoWriter
	}

	if len(vv) == 0 {
		return nil, ErrOutpSvcEmptyOut
	}

	os := new(OutputSvc)
	os.id = uuid.New()
	os.name = sname
	os.w = w
	os.v = append(os.v, vv...)

	return os, nil
}

// GetOutputService returns an Output Service or rise panic on error.
func GetOutputSvc(sname string, w io.Writer, vv ...interface{}) Service {
	srv, err := NewOutputSvc(sname, w, vv...)
	if err != nil {
		panic(fmt.Sprintf("couldn't create an Output Service : %v", err))
	}

	return srv
}

// Run runs Output Service.
//
// Run ignores its current state and starts over again even if
// it's already finished.
func (os *OutputSvc) Run(_ context.Context) error {
	if os.w == nil {
		os.state = SSFailed
		return ErrOutpSvcNoWriter
	}

	_, err := fmt.Fprint(os.w, os.v...)

	if err != nil {
		return ErrOutpSvcFailOutput
	}

	return nil
}

// ----------------------------------------------------------------------------

type PutMsgSvc struct {
	svcBase

	ms    *msgsrv.MessageServer
	qname string
	mm    []*msgsrv.Message
}

var ErrInvalidMsgSrv = errors.New("invalid message server pointer")

var ErrInvalidQueue = errors.New("invalid queue name")

var ErrPmsNoMessages = errors.New("no messages to put in")

// GetPutMessagesSvc calls NewPutMessagesSvc and if there is no error
// return the Service.
//
// If there is error, it will panic.
func GetPutMessageSvc(
	name string,
	ms *msgsrv.MessageServer,
	qname string,
	mm ...*msgsrv.Message) Service {

	pms, err := NewPutMessagesSvc(name, ms, qname, mm...)

	if err != nil {
		panic(err.Error())
	}

	return pms
}

// NewPutMessagesSvc creates a new Put Message servcie to put messages mm on
// message server ms in the queue qname.
func NewPutMessagesSvc(
	sname string,
	ms *msgsrv.MessageServer,
	qname string,
	mm ...*msgsrv.Message) (Service, error) {

	if sname == "" {
		sname = "PutMessages Service"
	}

	if ms == nil {
		return nil, ErrInvalidMsgSrv
	}

	if qname == "" {
		return nil, ErrInvalidQueue
	}

	if len(mm) == 0 {
		return nil, ErrPmsNoMessages
	}

	pms := new(PutMsgSvc)
	pms.id = uuid.New()
	pms.name = sname
	pms.qname = qname
	pms.ms = ms
	pms.mm = append(pms.mm, mm...)

	return pms, nil
}

// Run puts all messages from pms.mm into queue pms.qname on server pms.ms
func (pms *PutMsgSvc) Run(_ context.Context) error {

	mm := make([]msgsrv.Message, 0)
	for _, m := range pms.mm {
		mm = append(mm, *m)
	}

	return pms.ms.PutMessages(pms.qname, mm...)
}

// ----------------------------------------------------------------------------
type GetMgSvc struct {
	svcBase

	ms    *msgsrv.MessageServer
	qname string

	timeout  uint8
	attempts uint8

	mm        []*msgsrv.Message
	maxMsgNum uint8
}

// GetGetMessagesSvc calls NewGetMessagesSvc with the same parameters and
// returns the Service if there is no error.
//
// If there is an error during the Service creation, the panic will fired.
func GetGetMessagesSvc(
	name string,
	ms *msgsrv.MessageServer,
	qname string,
	timeout uint8,
	attempts uint8,
	maxMsgNum uint8) Service {

	gms, err := NewGetMessagesSvc(name, ms, qname,
		timeout, attempts, maxMsgNum)
	if err != nil {
		panic(err.Error())
	}

	return gms
}

// NewGetMessageSvc cretaes a GetMessages Service for getting msgsrv.Messages
// from MessageServer ms from queue qname.
//
// If there is no messages to read or there is no queue qname yet, the
// services tries to make the number of attemps sent in param attempts over
// timeout period in seconds between them.
// If timeout is 0 or attempts is 0 the Service will read messages only once.
//
// Once the number of gathered messages overcomes the maxMsgNum, then
// the Service stopped.
func NewGetMessagesSvc(
	name string,
	ms *msgsrv.MessageServer,
	qname string,
	timeout uint8,
	attempts uint8,
	maxMsgNum uint8) (Service, error) {

	if name == "" {
		name = "GetMessages Service"
	}

	if attempts == 0 {
		attempts = 1 // it should run at least once
	}

	if qname == "" {
		return nil, ErrInvalidQueue
	}

	if ms == nil {
		return nil, ErrInvalidMsgSrv
	}

	gms := new(GetMgSvc)
	gms.id = uuid.New()
	gms.name = name
	gms.ms = ms
	gms.qname = qname
	gms.timeout = timeout
	gms.attempts = attempts
	gms.maxMsgNum = maxMsgNum
	gms.mm = make([]*msgsrv.Message, 0)
	gms.resultsProvider =
		func() chan interface{} {
			res := make(chan interface{})

			go func() {
				for _, m := range gms.mm {
					res <- *m
				}

				close(res)
			}()

			return res
		}

	return gms, nil
}

// Run tries to get messages from queue gms.qname on server gms.ms.
//
// It waits untli the queue isn't present in given timeout and attempts.
// if the queue is present the Service tries to read msxMsgNum messages
// or all available messages if maxMsgNum is 0.
func (gms *GetMgSvc) Run(ctx context.Context) error {

	if gms.ms == nil || gms.qname == "" {
		return ErrInvalidMsgSrv
	}

	for i := gms.attempts; i > 0; i-- {
		// wait until the queue isn't ready
		if !gms.ms.HasQueue(gms.qname) {
			select {
			case <-ctx.Done():
				return ctx.Err()

			case <-time.After(time.Duration(gms.timeout) * time.Second):
				continue
			}
		}

		mm, err := gms.ms.GetMesages(gms.qname)
		if err != nil {
			return err
		}

		for _, m := range mm {
			gms.mm = append(gms.mm, &m)
			if gms.maxMsgNum != 0 && len(gms.mm) == int(gms.maxMsgNum) {
				return nil
			}
		}
	}

	return nil
}

// ----------------------------------------------------------------------------

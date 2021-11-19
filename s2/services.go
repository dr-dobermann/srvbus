package s2

import (
	"context"
	"errors"
	"fmt"
	"io"

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
	os.Name = sname
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

var ErrPmsInvalidMsgSrv = errors.New("invalid message server pointer")

var ErrPmsInvalidQueue = errors.New("invalid queue name")

var ErrPmsNoMessages = errors.New("no messages to put in")

// NewPutMessageSvc creates a new Put Message servcie to put messages mm on
// message server ms in the queue qname.
func NewPutMessageSvc(
	sname string,
	ms *msgsrv.MessageServer,
	qname string,
	mm ...*msgsrv.Message) (Service, error) {

	if sname == "" {
		sname = "PutMessage Service"
	}

	if ms == nil {
		return nil, ErrPmsInvalidMsgSrv
	}

	if qname == "" {
		return nil, ErrPmsInvalidQueue
	}

	if len(mm) == 0 {
		return nil, ErrPmsNoMessages
	}

	pms := new(PutMsgSvc)
	pms.id = uuid.New()
	pms.Name = sname
	pms.qname = qname
	pms.ms = ms
	pms.mm = append(pms.mm, mm...)

	return pms, nil
}

func (pms *PutMsgSvc) Run(_ context.Context) error {

	mm := make([]msgsrv.Message, 0)
	for _, m := range pms.mm {
		mm = append(mm, *m)
	}

	return pms.ms.PutMessages(pms.qname, mm...)
}

// ----------------------------------------------------------------------------

package s2

import (
	"context"
	"errors"
	"fmt"
	"io"

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

// type MsgPutSvc struct {
// 	svcBase

// 	ms *msgsrv.MessageServer
// 	mm []*msgsrv.Message
// }

// func NewPutMessageSvc(
// 	sname string,
// 	ms *msgsrv.MessageServer,
// 	mm ...msgsrv.Message) (Service, error) {

// 	pms := new(MsgPutSvc)

// 	return pms, nil
// }

// ----------------------------------------------------------------------------

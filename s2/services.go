package s2

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"
)

// OutputService simpy prints all its values on console.
type OutputSrv struct {
	srvBase

	w io.Writer
	v []interface{}
}

// NewOutputService creates a new Service based on OutputSrv.
//
// If Writer w is nil or vv lenght is zero, then error returned.
func NewOutputService(
	sname string,
	w io.Writer,
	vv ...interface{}) (Service, error) {

	if w == nil {
		return nil, NewSvcSrvError("", "nil-Writer for OutputService", nil)
	}

	if len(vv) == 0 {
		return nil, NewSvcSrvError("", "nothing to output for OutputService", nil)
	}

	os := new(OutputSrv)
	os.Id = uuid.New()
	os.Name = sname
	os.w = w
	os.v = append(os.v, vv...)

	return os, nil
}

// GetOutputService returns an Output Service or rise panic on error.
func GetOutputService(sname string, w io.Writer, vv ...interface{}) Service {
	srv, err := NewOutputService(sname, w, vv...)
	if err != nil {
		panic(fmt.Sprintf("couldn't create an Output Service : %v", err))
	}

	return srv
}

// Run runs Output Service.
//
// Run ignores its current state and starts over again even if
// it's already finished.
func (os *OutputSrv) Run(_ context.Context) error {
	if os.w == nil {
		os.state = SSFailed
		return NewSvcSrvError("", "cannot write to the nil Writer", nil)
	}

	if os.state != SSReady {
		return nil
	}

	os.Lock()
	defer os.Unlock()

	os.state = SSRunned

	fmt.Fprint(os.w, os.v...)

	os.state = SSFinished

	return nil
}

// ----------------------------------------------------------------------------

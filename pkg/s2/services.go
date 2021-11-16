package s2

import "context"

// OutputService simpy prints all its values on console.
type OutputSrv struct {
	srvInfo

	v []interface{}
}

func NewOutputService(sname string, vv ...interface{}) (Service, error) {
	os := new(OutputSrv)
	os.Name = sname
	os.v = append(os.v, vv...)

	return os, nil
}

func (os *OutputSrv) Run(_ context.Context) error {
	return nil
}

// ----------------------------------------------------------------------------

package s2

import (
	"context"
	"sync"
	"time"
)

type ServiceState uint8

const (
	SSCreated ServiceState = iota
	SSRunned
	SSAwaitsResponse
	SSFinished
	SSFailed
)

// srvInfo consists all the common Service information
type srvInfo struct {
	Name      string
	State     ServiceState
	LastError error
	m         *sync.Mutex
	nextCheck time.Time
}

type Service interface {
	Run(ctx context.Context) error
}

//-----------------------------------------------------------------------------

// ServiceServerError is a specialized error for module s2
type ServiceServerError struct {
	Sname   string
	Message string
	Err     error
}

func (err ServiceServerError) Error() string {
	em := "[" + err.Sname + "] " + err.Message

	if err.Err != nil {
		em += " : " + err.Err.Error()
	}

	return em
}

func NewSrvSrcError(sname string, msg string, err error) error {
	return ServiceServerError{sname, msg, err}
}

//-----------------------------------------------------------------------------

// ServiceServer represent state storage for Service Server instance.
type ServiceServer struct {
	Name string
}

// NewServiceServer creates a new ServiceServer and returns
// its pointer.
func NewServiceServer(sname string) *ServiceServer {
	if sname == "" {
		sname = "ServiceServer"
	}

	return &ServiceServer{Name: sname}
}

//-----------------------------------------------------------------------------

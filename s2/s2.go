package s2

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Service is an interface that every Service in s2 should implement.
type Service interface {
	// Run runs a service with cancelable context.
	Run(ctx context.Context) error

	// Returns current Service State
	State() ServiceState

	// UpdateTimer updates nextCheck if applicable.
	UpdateTimer()
}

type ServiceState uint8

const (
	SSReady ServiceState = iota
	SSRunned
	SSAwaitsResponse
	SSFinished
	SSFailed
)

// srvBase consists all the common Service information
type srvBase struct {
	sync.Mutex
	Id        uuid.UUID
	Name      string
	state     ServiceState
	timeout   time.Duration
	LastError error
	nextCheck time.Time
}

func (sb *srvBase) UpdateTimer() {

}

func (sb *srvBase) State() ServiceState {

	return sb.state
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

// NewSvcSrvError creates a new ServiceServerError
func NewSvcSrvError(sname string, msg string, err error) error {
	return ServiceServerError{sname, msg, err}
}

//-----------------------------------------------------------------------------

type ServerState uint8

const (
	SrvReady ServerState = iota
	SrvExecutingServices
	SrvStopped
)

func (ss ServerState) String() string {
	return []string{
		"Ready",
		"Executing Services",
		"Stopped",
	}[ss]
}

// ServiceServer represent state storage for Service Server instance.
type ServiceServer struct {
	sync.Mutex
	Name         string
	services     []Service
	ctx          context.Context
	state        ServerState
	lastError    error
	resetTimeout chan struct{}
}

// NewServiceServer creates a new ServiceServer and returns
// its pointer.
func NewServiceServer(sname string, ctx context.Context) *ServiceServer {
	if sname == "" {
		sname = "ServiceServer"
	}

	return &ServiceServer{
		Name:         sname,
		services:     make([]Service, 0),
		ctx:          ctx,
		resetTimeout: make(chan struct{})}
}

// AddService adds a non-nil Service s to ServiceServer ss.
func (ss *ServiceServer) AddService(s Service) error {
	if s == nil {
		return NewSvcSrvError(ss.Name, "couldn't add nil service on server", nil)
	}

	ss.Lock()
	// reset server's idle timeout
	if ss.state == SrvExecutingServices {
		ss.resetTimeout <- struct{}{}
	}

	ss.services = append(ss.services, s)
	ss.Unlock()

	return nil
}

// Start runs ServiceServer if is ready and there are registered Services on it.
func (ss *ServiceServer) Start(ctx context.Context) {
	if ss.state != SrvReady ||
		len(ss.services) == 0 {
		return
	}

	ss.Lock()
	defer ss.Unlock()

	ss.state = SrvExecutingServices

	go func(ctx context.Context) {
		// first start goes immediately
		timeout := time.Duration(0)
		for {
			select {
			case <-ctx.Done():
				ss.Lock()
				ss.state = SrvStopped
				ss.lastError = ctx.Err()
				ss.Unlock()
				return

			case <-ss.resetTimeout:
				timeout = time.Duration(0)

			case <-time.After(timeout * time.Second):
				ss.Lock()
				sc := 0
				for _, s := range ss.services {
					if s.State() != SSReady {
						continue
					}

					go func() {
						s.Run(ctx)
					}()

					sc++
				}
				ss.Unlock()

				// if there were no ready Services on the server
				// idle timeout sets to 10 seconds
				if sc == 0 {
					timeout = time.Duration(10 * time.Second)
				}
			}
		}
	}(ctx)
}

type ServerStatistics struct {
	SrvName       string
	State         ServerState
	Registered    int
	ResultsAwaits int
	Runned        int
	Ended         int
	Failed        int
}

func (ss ServerStatistics) String() string {
	return fmt.Sprintf(
		"Service Server %s Statistics\n"+
			"================================================\n"+
			"  State : %s\n"+
			"  Registered Services    : %d\n"+
			"  Runned Services        : %d\n"+
			"  Results Awaits Services: %d\n"+
			"  Ended Services         : %d\n"+
			"  Failed Services        : %d\n",
		ss.SrvName, ss.State.String(),
		ss.Registered, ss.Runned, ss.ResultsAwaits,
		ss.Ended, ss.Failed)
}

func (ss *ServiceServer) Stats() ServerStatistics {
	stat := new(ServerStatistics)

	ss.Lock()
	defer ss.Unlock()

	stat.SrvName = ss.Name
	stat.State = ss.state

	for _, s := range ss.services {
		stat.Registered++

		switch s.State() {
		case SSRunned:
			stat.Runned++

		case SSAwaitsResponse:
			stat.ResultsAwaits++

		case SSFinished:
			stat.Ended++

		case SSFailed:
			stat.Failed++
		}
	}

	return *stat
}

//-----------------------------------------------------------------------------

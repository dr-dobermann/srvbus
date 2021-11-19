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
	// Id returns Service's ID
	Id() uuid.UUID
	// Run runs a service with cancelable context.
	Run(ctx context.Context) error

	// Returns current Service State
	State() ServiceState

	// Sets current state for the Service
	SetState(ns ServiceState, srr error) error

	// ResetService resets the current state for Service to SSReady
	ResetService()

	// UpdateTimer updates nextCheck if applicable.
	UpdateTimer() time.Time
}

type ServiceState uint8

const (
	SSReady ServiceState = iota
	SSRunned
	SSAwaitsResponse
	SSFinished
	SSFailed
)

func (ss ServiceState) String() string {
	return []string{
		"Service Ready to run",
		"Service is Runned",
		"Service Awaits Results",
		"Service is Finished",
		"Service is Failed",
	}[ss]
}

type srvState struct {
	srvID uuid.UUID
	err   error
}

// svcBase consists all the common Service information
type svcBase struct {
	sync.Mutex
	id        uuid.UUID
	Name      string
	state     ServiceState
	timeout   time.Duration
	LastError error
	nextCheck time.Time
}

func (sb *svcBase) Id() uuid.UUID {
	return sb.id
}

// UpdateTimer calculates new time to start the Service.
//
// If timeout is 0, then 0 timer will be returned.
func (sb *svcBase) UpdateTimer() time.Time {
	if sb.timeout != 0 {
		sb.nextCheck = time.Now().Add(sb.timeout)
	}

	return sb.nextCheck
}

// State returns the state of the Service
func (sb *svcBase) State() ServiceState {
	sb.Lock()
	st := sb.state
	sb.Unlock()

	return st
}

// SetState sets new Service state ns.
//
// if ns is lesser then the current Service state, the error will be returned.
func (sb *svcBase) SetState(ns ServiceState, err error) error {
	sb.Lock()
	defer sb.Unlock()

	if ns <= sb.state {
		return NewSvcSrvError("", "infvalid new Service state "+ns.String()+
			". Sould be greater than "+sb.state.String(), nil)
	}

	sb.state = ns
	if err != nil {
		sb.LastError = err
	}

	return nil
}

// ResetService implements this method for svcBase
func (sb *svcBase) ResetService() {
	sb.Lock()
	defer sb.Unlock()

	sb.state = SSReady
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
	Name      string
	services  map[uuid.UUID]Service
	ctx       context.Context
	state     ServerState
	lastError error
}

// State returns current Server state
func (srv *ServiceServer) State() ServerState {

	return srv.state
}

// NewServiceServer creates a new ServiceServer and returns
// its pointer.
func NewServiceServer(sname string, ctx context.Context) *ServiceServer {
	if sname == "" {
		sname = "ServiceServer"
	}

	return &ServiceServer{
		Name:     sname,
		services: make(map[uuid.UUID]Service),
		ctx:      ctx}
}

// AddService adds a non-nil Service s to ServiceServer ss.
func (ss *ServiceServer) AddService(s Service) error {
	if s == nil {
		return NewSvcSrvError(ss.Name, "couldn't add nil service on server", nil)
	}

	ss.Lock()
	defer ss.Unlock()
	if _, ok := ss.services[s.Id()]; ok {
		return NewSvcSrvError(
			ss.Name,
			"service "+s.Id().String()+
				" already registerd on Server",
			nil)
	}

	ss.services[s.Id()] = s

	return nil
}

// Start runs ServiceServer if is ready and there are registered Services on it.
func (ss *ServiceServer) Start(ctx context.Context) error {
	ss.Lock()
	defer ss.Unlock()

	if ss.state != SrvReady ||
		len(ss.services) == 0 {
		return NewSvcSrvError(
			ss.Name,
			fmt.Sprintf("invalid server state %s or number of Services %d",
				ss.state.String(), len(ss.services)),
			nil)
	}
	ss.state = SrvExecutingServices

	sChan := make(chan srvState)

	// start service monitor to mark broken services
	// if it returns non-nil error
	go func() {
		for {
			select {
			case s := <-sChan:
				st := SSFinished
				if s.err != nil {
					_, ok := ss.services[s.srvID]
					if !ok {
						panic("service " + s.srvID.String() +
							" not found on Server " + ss.Name)
					}

					st = SSFailed
				}
				ss.services[s.srvID].SetState(st, s.err)

			case <-ctx.Done():
				close(sChan)
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				ss.state = SrvStopped
				ss.lastError = ctx.Err()
				return

			case <-time.After(1 * time.Second):
				for _, s := range ss.services {
					svc := s
					if svc.State() == SSReady {
						svc.SetState(SSRunned, nil)
						go func() {
							err := svc.Run(ctx)
							sChan <- srvState{svc.Id(), err}
						}()
					}
				}

			}
		}
	}()

	return nil
}

// ServerStatistics gather and returns the server statistics
func (ss *ServiceServer) Stats() ServerStatistics {
	stat := new(ServerStatistics)

	stat.SrvName = ss.Name
	stat.State = ss.state

	for _, s := range ss.services {
		stat.Registered++

		st := s.State()
		switch st {
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

// ServerStatistics represents status information.
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

//-----------------------------------------------------------------------------

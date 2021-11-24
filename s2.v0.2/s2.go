package s2

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

var ErrNoResultProvidr = errors.New("resultsProvider isn't set for the Service")

var ErrInvalidSvcState = errors.New("couldn't get results from failed service")

// Uploader is a functor user should provide to upload results
// from the Service.
//
// Uploader will run by the Service and results will provide over
// the channel results.
type Uploader func(results <-chan interface{}) error

// Service is an interface that every Service in s2 should implement.
type Service interface {
	// Id returns Service's ID
	Id() uuid.UUID

	Name() string

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

	UploadResults(ctx context.Context, uploader Uploader) error
}

type ServiceState uint8

const (
	SSReady ServiceState = iota
	SSRunned
	SSFinished
	SSFailed
)

func (ss ServiceState) String() string {
	return []string{
		"Service Ready to run",
		"Service is Runned",
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
	name      string
	state     ServiceState
	timeout   time.Duration
	LastError error
	nextCheck time.Time

	// resultsProvider ia s function with returns a channel of
	// the Service results
	resultsProvider func() chan interface{}
}

func (sb *svcBase) Id() uuid.UUID {

	return sb.id
}

func (sb *svcBase) Name() string {

	return sb.name
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

// UploadResults runs an
func (sb *svcBase) UploadResults(ctx context.Context, uploader Uploader) error {

	if sb.resultsProvider == nil {
		return ErrNoResultProvidr
	}

	ready := make(chan bool)
	defer close(ready)

	// wait for Service readiness
	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				ready <- false
			default:
			}

			sb.Lock()
			st := sb.state
			sb.Unlock()
			switch st {
			case SSRunned:
				time.Sleep(10 * time.Millisecond)
				continue

			case SSFailed:
				ready <- false
				return

			case SSFinished:
				ready <- true
				return
			}
		}
	}(ctx)

	select {
	case <-ctx.Done():
		return ctx.Err()

	case rdy := <-ready:
		if !rdy {
			return ErrInvalidSvcState
		}
	}

	return uploader(sb.resultsProvider())
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

// Error implements error interface for ServiceServerError
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
		for ss.state == SrvExecutingServices {
			select {
			case <-ctx.Done():
				ss.state = SrvStopped
				ss.lastError = ctx.Err()
				return

			case <-time.After(1 * time.Second):
				for _, s := range ss.services {
					s := s
					if s.State() == SSReady {
						s.SetState(SSRunned, nil)
						go func() {
							err := s.Run(ctx)
							sChan <- srvState{s.Id(), err}
						}()
					}
				}

			}
		}
	}()

	return nil
}

// Stops trying to stop the ServiceServier after all Services
// finished or failed.
//
// if timeout for stipping is passed, then error returns.
func (ss *ServiceServer) Stop(timeout time.Duration) error {

	limit := time.Now().Add(timeout)
	for time.Now().Before(limit) {
		if ss.canStop() {
			ss.Lock()
			ss.state = SrvStopped
			ss.Unlock()

			return nil
		}
	}
	return NewSvcSrvError(ss.Name, "timeout for stopping exceeded", nil)
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
			stat.RunnedSvc = append(stat.RunnedSvc, s.Name())

		case SSFinished:
			stat.Ended++
			stat.EndedSvc = append(stat.EndedSvc, s.Name())

		case SSFailed:
			stat.Failed++
			stat.FailedSvc = append(stat.FailedSvc, s.Name())
		}
	}

	return *stat
}

func (ss *ServiceServer) canStop() bool {

	return ss.Stats().Runned == 0
}

// ServerStatistics represents status information.
type ServerStatistics struct {
	SrvName    string
	State      ServerState
	Registered int
	Runned     int
	Ended      int
	Failed     int
	RunnedSvc  []string
	EndedSvc   []string
	FailedSvc  []string
}

func (ss ServerStatistics) String() string {
	return fmt.Sprintf(
		"Service Server %s Statistics\n"+
			"================================================\n"+
			"  State : %s\n"+
			"  Registered Services    : %d\n"+
			"  Runned Services        : %d (%v)\n"+
			"  Ended Services         : %d (%v)\n"+
			"  Failed Services        : %d (%v)\n",
		ss.SrvName, ss.State.String(),
		ss.Registered,
		ss.Runned, ss.RunnedSvc,
		ss.Ended, ss.EndedSvc,
		ss.Failed, ss.FailedSvc)
}

//-----------------------------------------------------------------------------
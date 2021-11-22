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
	ID() uuid.UUID

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
	SSPrepared ServiceState = iota
	SSReady
	SSRunned
	SSFinished
	SSFailed
)

func (ss ServiceState) String() string {
	return []string{
		"Service is prepared to start",
		"Service Ready to run",
		"Service is Runned",
		"Service is Finished",
		"Service is Failed",
	}[ss]
}

type svcState struct {
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

func (sb *svcBase) ID() uuid.UUID {
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

	const readinessWaitTime = 10

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
				time.Sleep(readinessWaitTime * time.Millisecond)

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

	// wait for service readiness reply above
	// or cancel of Context.
	select {
	case <-ctx.Done():
		return fmt.Errorf("services stopped by context.cancel %w", ctx.Err())

	case rdy := <-ready:
		if !rdy {
			return ErrInvalidSvcState
		}
	}

	// upload all results from the Service
	// into gathering function.
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
			". Could be greater than "+sb.state.String(), nil)
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
func NewServiceServer(ctx context.Context, sname string) *ServiceServer {
	if sname == "" {
		sname = "ServiceServer"
	}

	return &ServiceServer{
		Name:     sname,
		services: make(map[uuid.UUID]Service),
		ctx:      ctx}
}

// AddService adds a non-nil Service s to ServiceServer srv.
func (srv *ServiceServer) AddService(s Service) error {
	if s == nil {
		return NewSvcSrvError(srv.Name, "couldn't add nil service on server", nil)
	}

	srv.Lock()
	defer srv.Unlock()

	if _, ok := srv.services[s.ID()]; ok {
		return NewSvcSrvError(
			srv.Name,
			"service "+s.ID().String()+
				" already registered on Server",
			nil)
	}

	srv.services[s.ID()] = s

	return nil
}

// svcEndingWaiter waits for Servcie completion and sets it state accordingly.
func (srv *ServiceServer) svcEndingWaiter(ctx context.Context, sChan chan svcState) {
	for {
		select {
		case s := <-sChan:
			st := SSFinished

			if s.err != nil {
				_, ok := srv.services[s.srvID]
				if !ok {
					panic("service " + s.srvID.String() +
						" not found on Server " + srv.Name)
				}

				st = SSFailed
			}

			if err := srv.services[s.srvID].SetState(st, s.err); err != nil {
				panic("couldn't set state for service " + s.srvID.String())
			}

		case <-ctx.Done():
			close(sChan)

			return
		}
	}
}

func (srv *ServiceServer) svcExecutor(ctx context.Context, sChan chan svcState) {
	for srv.state == SrvExecutingServices {
		select {
		case <-ctx.Done():
			srv.state = SrvStopped
			srv.lastError = ctx.Err()

			return

		case <-time.After(1 * time.Second):
			for _, s := range srv.services {
				s := s

				if s.State() == SSReady {
					if err := s.SetState(SSRunned, nil); err != nil {
						panic("couldn't set state for service " +
							s.ID().String())
					}

					go func() {
						err := s.Run(ctx)
						sChan <- svcState{s.ID(), err}
					}()
				}
			}
		}
	}
}

// Start runs ServiceServer if is ready and there are registered Services on it.
func (srv *ServiceServer) Start(ctx context.Context) error {
	srv.Lock()
	defer srv.Unlock()

	if srv.state != SrvReady ||
		len(srv.services) == 0 {
		return NewSvcSrvError(
			srv.Name,
			fmt.Sprintf("invalid server state %s or number of Services %d",
				srv.state.String(), len(srv.services)),
			nil)
	}

	srv.state = SrvExecutingServices

	sChan := make(chan svcState)

	// start service monitor to mark Failed or Ended services
	// if it returns non-nil error
	go srv.svcEndingWaiter(ctx, sChan)

	go srv.svcExecutor(ctx, sChan)

	return nil
}

// Stops trying to stop the ServiceServier after all Services
// finished or failed.
//
// if timeout for stopping is passed, then error returns.
func (srv *ServiceServer) Stop(ctx context.Context, timeout time.Duration) error {

	timer := time.After(timeout)

	for !srv.canStop() {
		select {
		case <-ctx.Done():
			return fmt.Errorf("server %s stopped by context : %w", srv.Name, ctx.Err())

		case <-timer:
			return NewSvcSrvError(srv.Name, "timeout for stopping exceeded", nil)

		default:

		}
	}

	srv.Lock()
	srv.state = SrvStopped
	srv.Unlock()

	return nil
}

// ServerStatistics gather and returns the server statistics
func (srv *ServiceServer) Stats() ServerStatistics {
	stat := new(ServerStatistics)

	stat.SrvName = srv.Name
	stat.State = srv.state

	for _, s := range srv.services {
		stat.Registered++

		st := s.State()

		switch st {
		case SSPrepared:
			stat.Prepared++

		case SSReady:
			stat.Ready++

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

func (srv *ServiceServer) canStop() bool {

	return len(srv.services) == 0 || srv.Stats().Runned == 0
}

// ServerStatistics represents status information.
type ServerStatistics struct {
	SrvName    string
	State      ServerState
	Registered int
	Prepared   int
	Ready      int
	Runned     int
	Ended      int
	Failed     int
	RunnedSvc  []string
	EndedSvc   []string
	FailedSvc  []string
}

func (srv ServerStatistics) String() string {
	return fmt.Sprintf(
		"Service Server %s Statistics\n"+
			"================================================\n"+
			"  State : %s\n"+
			"  Registered Services       : %d\n"+
			"  Services not ready to run : %d\n"+
			"  Services, ready to run    : %d\n"+
			"  Runned Services           : %d (%v)\n"+
			"  Ended Services            : %d (%v)\n"+
			"  Failed Services           : %d (%v)\n",
		srv.SrvName, srv.State.String(),
		srv.Registered,
		srv.Prepared,
		srv.Ready,
		srv.Runned, srv.RunnedSvc,
		srv.Ended, srv.EndedSvc,
		srv.Failed, srv.FailedSvc)
}

//-----------------------------------------------------------------------------

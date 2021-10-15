package srvbus

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ServiceState uint8

const (
	SSCreated ServiceState = iota
	SSStarted
	SSChecking
	SSFinished
	SSBroken
	SSAwaitsResponse
)

type SrvFunc func(ctx context.Context, s *Service) error

type Service struct {
	id        uuid.UUID
	state     ServiceState
	lastError error
	sfunc     SrvFunc
	m         *sync.Mutex
	// if the Service in the AwaitResponse, nextCheck holds the time to
	// check the results again
	nextCheck time.Time
	params    []interface{}
	results   []interface{}
}

func (s *Service) SetState(ss ServiceState) {
	if s.state == SSFinished || s.state == SSBroken {
		return
	}

	s.m.Lock()
	s.state = ss
	s.m.Unlock()
}

type ServiceErrorWrapper struct {
	Srvc    *Service
	Message string
}

func (err ServiceErrorWrapper) Error() string {
	if err.Srvc == nil {
		return err.Message
	}

	return fmt.Sprintf("%v: %s", err.Srvc.id, err.Message)
}

func NewServiceError(s *Service, msg string) error {
	return ServiceErrorWrapper{s, msg}
}

type ServiceServer struct {
	started  bool
	services map[uuid.UUID]*Service
}

func NewServiceServer() *ServiceServer {
	return &ServiceServer{false, make(map[uuid.UUID]*Service)}
}

func (srv *ServiceServer) AddTask(sf SrvFunc, p ...interface{}) (uuid.UUID, error) {
	var uid uuid.UUID

	if sf == nil {
		log.Printf("Attempting to create a service with an empty service function")
		return uid, NewServiceError(nil, "couldn't add service with an empty service function")
	}

	uid = uuid.New()
	srv.services[uid] = &Service{id: uid, state: SSCreated, sfunc: sf, m: new(sync.Mutex), params: p}

	log.Printf("New service %v added to the queue", uid)

	return uid, nil
}

func (srv *ServiceServer) Run(ctx context.Context) {
	if srv.started {
		return
	}

	log.Print("Service bus server started.")

	// start service monitor to mark broken services
	// if it returns non-nil error
	sChan := make(chan srvState)
	go func() {
		for {
			select {
			case s := <-sChan:
				if s.err != nil {
					srv.services[s.srvID].SetState(SSBroken)
					srv.services[s.srvID].lastError = s.err
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Main cycle for starting services every 1 second
	// if it doesn't have their own timeout
	go func() {
		for {
			select {
			case <-time.After(1 * time.Second):
				log.Print("Service queue processing started")
				for _, s := range srv.services {
					if s.state == SSFinished || s.state == SSBroken || s.state == SSStarted {
						continue
					}

					if time.Now().Before(s.nextCheck) {
						log.Printf("Time isn't come for service %v", s.id)
						continue
					}

					go runService(ctx, s, sChan)
				}
				log.Print("Service queue processing ended")
			case <-ctx.Done():
				srv.started = false
				log.Print("Service bus server ended.")
				return
			}
		}
	}()

	srv.started = true
}

type srvState struct {
	srvID uuid.UUID
	err   error
}

// helper function to cover service function call
func runService(ctx context.Context, srv *Service, state chan srvState) {
	srv.SetState(SSStarted)

	log.Printf("Starting service %v...", srv.id)
	err := srv.sfunc(ctx, srv)
	state <- srvState{srv.id, err}
	log.Printf("Service %v ended. Status: %v, Error: %v", srv.id, srv.state, err)
}

func (srv *ServiceServer) ListServices() {
	fmt.Println("srv ID, state, nextCheck, lastError")

	for _, s := range srv.services {
		fmt.Println(s.id, s.state, s.nextCheck, s.lastError)
	}
}

func (srv *ServiceServer) DelService(sid uuid.UUID) error {
	if _, ok := srv.services[sid]; ok {
		if srv.services[sid].state == SSStarted {
			return NewServiceError(srv.services[sid], "Couldn't delete executing service")
		}

		log.Printf("Service %v deleted from the queue", sid)
		delete(srv.services, sid)
	} else {
		return NewServiceError(nil, fmt.Sprintf("couldn't find service %v", sid))
	}

	return nil
}

func (srv *ServiceServer) GetStatus(sid uuid.UUID) (ServiceState, error, error) {
	if _, ok := srv.services[sid]; !ok {
		return SSBroken, nil, NewServiceError(nil, fmt.Sprintf("couldn't find service %v", sid))
	}

	return srv.services[sid].state, srv.services[sid].lastError, nil
}

func (srv *ServiceServer) GetResults(sid uuid.UUID, whateverHave bool) ([]interface{}, error) {
	if _, ok := srv.services[sid]; !ok {
		return nil, NewServiceError(nil, fmt.Sprintf("couldn't find service %v", sid))
	}

	srv.services[sid].m.Lock()
	defer srv.services[sid].m.Unlock()

	if srv.services[sid].state != SSFinished && !whateverHave {
		return nil, NewServiceError(srv.services[sid],
			fmt.Sprintf("Service isn't finished (current state: %v)", srv.services[sid].state))
	}

	return srv.services[sid].results, nil
}

// SrvOutput prints its parameters into console
func SrvOutput(ctx context.Context, s *Service) error {
	fmt.Println(s.params...)

	s.SetState(SSFinished)

	return nil
}

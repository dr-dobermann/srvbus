package srvbus

import (
	"context"
	"fmt"
	"log"
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
	// if the Service in the AwaitResponse, nextCheck is the time to
	// check the results
	nextCheck time.Time
	params    []interface{}
	results   []interface{}
}

func NewServiceServer() *ServiceServer {
	return &ServiceServer{false, make(map[uuid.UUID]*Service)}
}

type ServiceServer struct {
	started  bool
	services map[uuid.UUID]*Service
}

func (srv *ServiceServer) AddTask(sf SrvFunc, p ...interface{}) (uuid.UUID, error) {
	var uid uuid.UUID

	if sf == nil {
		log.Printf("Attempting to create a service with an empty service function")
		return uid, fmt.Errorf("couldn't add service with an empty service function")
	}

	uid = uuid.New()
	srv.services[uid] = &Service{id: uid, state: SSCreated, sfunc: sf, params: p}
	log.Printf("New service %v added to the queue", uid)

	return uid, nil
}

func (srv *ServiceServer) Run(ctx context.Context) {
	if srv.started {
		return
	}
	log.Print("Service bus server started.")
	sChan := make(chan srvState)
	go func() {
		for {
			select {
			case s := <-sChan:
				if s.err != nil {
					srv.services[s.srvID].state = SSBroken
					srv.services[s.srvID].lastError = s.err
				}
			case <-ctx.Done():
				return
			}
		}
	}()
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

func runService(ctx context.Context, srv *Service, state chan srvState) {
	srv.state = SSStarted
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
			return fmt.Errorf("couldn't delete executing service %v", sid)
		}
		log.Printf("Service %v deleted from the queue", sid)
		delete(srv.services, sid)
	} else {
		return fmt.Errorf("couldn't find service %v", sid)
	}
	return nil
}

func (srv *ServiceServer) GetStatus(sid uuid.UUID) (ServiceState, error, error) {
	if _, ok := srv.services[sid]; ok {
		return SSBroken, nil, fmt.Errorf("couldn't find service wtih id %v", sid)
	}
	return srv.services[sid].state, srv.services[sid].lastError, nil
}

func (srv *ServiceServer) GetResults(sid uuid.UUID) ([]interface{}, error) {
	if _, ok := srv.services[sid]; !ok {
		return nil, fmt.Errorf("couldn't find service wtih id %v", sid)
	}
	if srv.services[sid].state != SSFinished {
		return nil, fmt.Errorf("service %v isn't finished (current state: %v)", sid, srv.services[sid].state)
	}

	return srv.services[sid].results, nil
}

func SrvMsgOutput(ctx context.Context, s *Service) error {
	fmt.Println(s.params...)
	s.state = SSFinished
	return nil
}

// srvBus is a Service Providing Server developed to
// support project GoBPM.
//
// (c) 2021, Ruslan Gabitov a.k.a. dr-dobermann.
// Use of this source is governed by LGPL license that
// can be found in the LICENSE file.
//
/*
Package s2 is a part of the srvbus package. s2 consists of the
in-memory Service Server implementation.

Service Server is registering, running and monitoring Services
needed for its users.
*/
package s2

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// =============================================================================
// ServiceRunner present interface for service execution.
type ServiceRunner interface {
	Run(ctx context.Context) error
}

// ServiceFunc could be used in case there is no need to
// keep service state.
type ServiceFunc func(ctx context.Context) error

// Run implements ServiceRunner interface for the
// ServiceFunc
func (sf ServiceFunc) Run(ctx context.Context) error {
	return sf(ctx)
}

// =============================================================================
// serviceRecord holds information about single Service registered on the
// Service Server
type serviceRecord struct {
	id        uuid.UUID
	name      string
	svc       ServiceRunner
	state     svcState
	lastError error
}

// svcState presents current Service state.
type svcState uint8

const (
	SSRegistered svcState = iota
	SSReady
	SSRunning
	SSEnded
	SSFailed
)

func (s svcState) String() string {
	return []string{
		"Registered",
		"Ready",
		"Running",
		"Ended",
		"Failed",
	}[s]
}

// =============================================================================
// ServiceServer holds the Service Server current state.
type ServiceServer struct {
	sync.Mutex

	ID   uuid.UUID
	Name string
	log  *zap.SugaredLogger

	services   map[uuid.UUID]*serviceRecord
	svcResCh   chan svcRes
	cvcReadyCh chan uuid.UUID
}

// svcRes consists of results of execution one single Service.
type svcRes struct {
	id  uuid.UUID
	err error
}

// New creates a new ServiceServer and returns its pointer.
func New(
	id uuid.UUID,
	name string,
	log *zap.SugaredLogger) (*ServiceServer, error) {
	if log == nil {
		return nil, fmt.Errorf("logger isn't presented")
	}

	if id == uuid.Nil {
		id = uuid.New()
	}

	if name == "" {
		name = "ServiceServer #" + id.String()
	}

	sSrv := new(ServiceServer)
	sSrv.ID = id
	sSrv.Name = name
	sSrv.log = log
	sSrv.services = make(map[uuid.UUID]*serviceRecord)

	sSrv.log.Debugw("new Service Server created",
		"id", sSrv.ID,
		"name", sSrv.Name)

	return sSrv, nil
}

// regSvcState awaits the Service to finish and sets its
// finish state.
//
// It runs as a go-routine from the Run method.
//
func (sSrv *ServiceServer) regSvcState(ctx context.Context) {
	sSrv.log.Debugw("service state registrator started",
		"srvID", sSrv.ID)

	for {
		select {
		case <-ctx.Done():
			sSrv.Lock()

			close(sSrv.svcResCh)
			close(sSrv.cvcReadyCh)

			sSrv.svcResCh = nil
			sSrv.cvcReadyCh = nil

			sSrv.Unlock()

			return

		case sRes, ok := <-sSrv.svcResCh:
			if !ok { // return on closed channel
				return
			}

			s := SSEnded
			if sRes.err != nil {
				s = SSFailed
			}

			sSrv.Lock()

			sr, ok := sSrv.services[sRes.id]
			if !ok {
				sSrv.log.Errorw("couldn't find service",
					"srvID", sSrv.ID,
					"service id", sRes.id)

				sSrv.Unlock()

				continue
			}

			sr.state = s
			sr.lastError = sRes.err

			sSrv.Unlock()

			sSrv.log.Debugw("service ended",
				"srvID", sSrv.ID,
				"svc id", sRes.id,
				"svc name", sr.name,
				"state", s.String(),
				"error", sRes.err)

		// validity of the id should be checked before
		// putting it into the channel. If not it causes panic
		case id := <-sSrv.cvcReadyCh:
			sSrv.Lock()
			sSrv.services[id].state = SSReady
			sSrv.Unlock()

			sSrv.log.Debugw("service is ready to start",
				"srvID", sSrv.ID,
				"service ID", id)
		}
	}
}

// loop executes main processing loop of the Service Server.
//
// loop is started from Run method.
func (sSrv *ServiceServer) loop(ctx context.Context) {
	sSrv.log.Debugw("service server operation loop started",
		"srvID", sSrv.ID)

	for {
		// check if context cancelled
		select {
		case <-ctx.Done():
			sSrv.log.Debugw("server stopped",
				"id", sSrv.ID,
				"name", sSrv.Name)

			return

		default:
		}

		sSrv.Lock()

		for id, sr := range sSrv.services {
			sr := sr
			if sr.state == SSReady {
				sr.state = SSRunning

				go func() {
					sSrv.log.Debugw("service started",
						"srvID", sSrv.ID,
						"service ID", id,
						"service name", sr.name)

					err := sr.svc.Run(ctx)

					if sSrv.svcResCh == nil {
						sSrv.log.Errorw("service result channel is closed",
							"srvID", sSrv.ID,
							"service ID", id)

						return
					}

					sSrv.svcResCh <- svcRes{id, err}
				}()
			}
		}
		sSrv.Unlock()
	}
}

// Run starts the Service Server's processing cycle
//
// to stop the server just call canecel function of the
// context.
func (sSrv *ServiceServer) Run(ctx context.Context) error {
	sSrv.log.Debugw("server started",
		"id", sSrv.ID,
		"name", sSrv.Name)

	// creating channels every time server runs provides
	// ability of multi run-stop execution.
	sSrv.svcResCh = make(chan svcRes)
	sSrv.cvcReadyCh = make(chan uuid.UUID)

	go sSrv.regSvcState(ctx)

	go sSrv.loop(ctx)

	return nil
}

// Add registers service on the server and returns its ID.
func (sSrv *ServiceServer) AddService(
	name string,
	s ServiceRunner) (uuid.UUID, error) {
	if s == nil {
		return uuid.Nil, fmt.Errorf("couldn't register nil-service")
	}

	id := uuid.New()

	if name == "" {
		name = "Service #" + id.String()
	}

	sSrv.Lock()
	sSrv.services[id] = &serviceRecord{
		id:    id,
		name:  name,
		state: SSRegistered,
		svc:   s}
	sSrv.Unlock()

	sSrv.log.Debugw("new service registered",
		"id", id,
		"name", name)

	return id, nil
}

// ExecService sends meassge to update Service state
// from SSRegistered to SSReady so it could start.
func (sSrv *ServiceServer) ExecService(id uuid.UUID) error {
	sSrv.Lock()
	sr, ok := sSrv.services[id]
	defer sSrv.Unlock()

	if !ok {
		sSrv.log.Errorw("service isn't found",
			"srvID", sSrv.ID,
			"service ID", id)

		return fmt.Errorf("couldn't find service # %v", id)
	}

	if sr.state != SSRegistered {
		sSrv.log.Errorw("unexpectd service state",
			"srvID", sSrv.ID,
			"service ID", id,
			"state", sr.state.String())

		return fmt.Errorf("invalid %s service state state %s for running",
			id.String(), sr.state.String())
	}

	sSrv.cvcReadyCh <- id

	return nil
}

// WaitForService waits for finalization one or all Serives on
// the Server.
//
// If id is uuid.Nil then it waits for finalization of all Services on the
// Server.
//nolint:gocognit, cyclop
func (sSrv *ServiceServer) WaitForService(
	ctx context.Context,
	id uuid.UUID) chan error {
	sSrv.log.Debugw("wait for service",
		"srvID", sSrv.ID,
		"service ID", id)

	resChan := make(chan error, 1)

	if id != uuid.Nil {
		sSrv.Lock()
		_, ok := sSrv.services[id]
		sSrv.Unlock()

		if !ok {
			resChan <- fmt.Errorf("couldn't find service %s", id.String())
			close(resChan)
			sSrv.log.Errorw("service isn't found for wait",
				"srvID", sSrv.ID,
				"service ID", id)

			return resChan
		}
	}

	go func() {
		for {
			// check context
			select {
			case <-ctx.Done():
				resChan <- ctx.Err()
				close(resChan)

				return

			default:
			}

			cnt := 0

			sSrv.Lock()

			for _, rs := range sSrv.services {
				if rs.state == SSEnded || rs.state == SSFailed {
					cnt++

					// check finalization of one Service
					if id != uuid.Nil && rs.id == id {
						resChan <- nil
						close(resChan)
						sSrv.Unlock()

						return
					}
				}
			}

			// check all services ended
			if id == uuid.Nil && cnt == len(sSrv.services) {
				resChan <- nil
				close(resChan)
				sSrv.Unlock()

				return
			}

			sSrv.Unlock()
		}
	}()

	return resChan
}

// S2Stat consists the Service Server status returned by Stat mehtod.
type S2Stat struct {
	Name     string
	ID       uuid.UUID
	Active   bool
	Services int
	Splits   map[string]Info
}

// String implements the fmt.Stringer interface and
// returns the string representation of the S2Stat
func (stat S2Stat) String() string {
	res := "\nService Server\n" +
		"======================================\n" +
		"Name          : " + stat.Name + "\n" +
		"ID            : " + stat.ID.String() + "\n"

	if stat.Active {
		res += "Status        : Active\n"
	} else {
		res += "Status        : Inactive\n"
	}
	res += "Total Services: " + strconv.Itoa(stat.Services) + "\n"

	res += "======================================\n"

	for st, info := range stat.Splits {
		res += "  [" + st + "] " + info.Svcs[0] + "\n"
		for _, s := range info.Svcs[1:] {
			res += strings.Repeat(" ", len(st)+5) + s + "\n"
		}
	}

	res += "\n"

	return res
}

// Info holds information about services with the same state
type Info struct {
	Num  int
	Svcs []string
}

func (sSrv *ServiceServer) Stat() S2Stat {
	stat := new(S2Stat)
	stat.Splits = make(map[string]Info)

	sSrv.Lock()
	defer sSrv.Unlock()

	stat.Active = sSrv.svcResCh != nil
	stat.Name = sSrv.Name
	stat.ID = sSrv.ID

	for _, sr := range sSrv.services {
		stat.Services++

		info := stat.Splits[sr.state.String()]
		info.Num++
		info.Svcs = append(info.Svcs, sr.id.String()+" : "+sr.name)
		stat.Splits[sr.state.String()] = info
	}

	return *stat
}

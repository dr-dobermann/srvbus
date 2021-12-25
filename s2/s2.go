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

Service Server is registering, running and monitoring external
and internalServices needed for its users.

It could be used separately from the others srvbus pacackages.
*/
package s2

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/dr-dobermann/srvbus/es"
	"github.com/dr-dobermann/srvbus/internal/errs"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	srvNew   = "NEW_SSERVER_EVT"
	srvStart = "SSERVER_START_EVT"
	srvEnd   = "SSERVER_END_EVT"

	svcStart = "SERVICE_START_EVT"
	svcEnd   = "SERVICE_END_EVT"

	defaultTopic = "/s2"
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
	sync.Mutex

	id          uuid.UUID
	name        string
	svc         ServiceRunner
	state       svcState
	lastError   error
	stopChannel chan struct{}
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

func (sr *serviceRecord) getState() svcState {
	sr.Lock()
	defer sr.Unlock()

	return sr.state
}

func (sr *serviceRecord) setState(ns svcState, err error) {
	sr.Lock()
	defer sr.Unlock()

	sr.state = ns
	sr.lastError = err
}

// =============================================================================
// ServiceServer holds the Service Server current state.
type ServiceServer struct {
	sync.Mutex

	ID   uuid.UUID
	Name string
	log  *zap.SugaredLogger

	ctx context.Context

	services map[uuid.UUID]*serviceRecord
	svcRunCh chan uuid.UUID

	runned bool

	eSrv    *es.EventServer
	esTopic string
}

func (sSrv *ServiceServer) IsRunned() bool {
	sSrv.Lock()
	defer sSrv.Unlock()

	return sSrv.runned
}

// emits single event into the internal event server topic
// if the Event Server was given on New call.
func (sSrv *ServiceServer) emitEvent(name, descr string) {
	if sSrv.eSrv == nil || !sSrv.eSrv.IsRunned() {
		sSrv.log.Warnw("couldn't register event on non-runned event server",
			zap.String("name", name))

		return
	}

	// initialize default server topic if needed
	if sSrv.esTopic == "" {
		topic := defaultTopic + "/" + sSrv.ID.String()
		if err := sSrv.eSrv.AddTopicQueue(topic, es.RootTopic); err != nil {
			sSrv.log.Warnw("couldn't add topic to Event Server",
				zap.String("eSrvName", sSrv.eSrv.Name),
				zap.Stringer("eSrvID", sSrv.eSrv.ID),
				zap.String("topic", topic),
				zap.Error(err))
			return
		}
		sSrv.esTopic = topic
	}

	es.EmitEvt(sSrv.eSrv, sSrv.esTopic, name, descr, sSrv.ID)
}

// New creates a new ServiceServer and returns its pointer.
func New(
	id uuid.UUID,
	name string,
	log *zap.SugaredLogger,
	eSrv *es.EventServer) (*ServiceServer, error) {

	if log == nil {
		return nil, errs.ErrNoLogger
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
	sSrv.log = log.Named("S2 [" + sSrv.Name +
		"] #" + sSrv.ID.String())
	sSrv.services = make(map[uuid.UUID]*serviceRecord)
	sSrv.eSrv = eSrv

	sSrv.emitEvent(srvNew, "{name: \""+sSrv.Name+"\"}")

	sSrv.log.Info("service server created")

	return sSrv, nil
}

// loop executes main processing loop of the Service Server.
//
// loop is started from Run method.
func (sSrv *ServiceServer) loop(ctx context.Context) {
	sSrv.log.Debug("service state registrator started")

	for {
		select {
		case <-ctx.Done():
			sSrv.Lock()
			sSrv.runned = false
			sSrv.Unlock()

			sSrv.log.Info("service server ended")

			sSrv.emitEvent(srvEnd, "")

			return

		case id := <-sSrv.svcRunCh:
			sSrv.Lock()
			sr, ok := sSrv.services[id]
			sSrv.Unlock()

			if !ok {
				sSrv.log.Warnw("service not found",
					zap.Stringer("svc_id", id))

				continue
			}

			if sr.getState() != SSReady {

				continue
			}

			go func() {
				sr.setState(SSRunning, nil)

				sSrv.emitEvent(svcStart,
					fmt.Sprintf(
						"{name: \"%s\", id: \"%v\"}",
						sr.name, sr.id))

				sSrv.log.Info("service started")

				err := sr.svc.Run(ctx)

				if err != nil {
					sr.setState(SSFailed, err)

					sSrv.log.Infow("service failed",
						zap.Stringer("svc_id", id),
						zap.Error(err))

					return
				}

				sr.setState(SSEnded, nil)

				sSrv.emitEvent(svcEnd,
					fmt.Sprintf(
						"{name: \"%s\", id: \"%v\", state: \"%d:%s\"}",
						sr.name, sr.id, sr.state, sr.state.String()))

				sSrv.log.Infow("service ended",
					zap.Stringer("svc_id", id))
			}()
		}
	}
}

// Run starts the Service Server's processing cycle
//
// to stop the server just call canecel function of the
// context.
func (sSrv *ServiceServer) Run(ctx context.Context) error {
	if sSrv.IsRunned() {
		return errs.ErrAlreadyRunned
	}

	sSrv.log.Info("server starting...")

	sSrv.Lock()
	sSrv.runned = true
	sSrv.ctx = ctx
	sSrv.svcRunCh = make(chan uuid.UUID)
	sSrv.Unlock()

	go sSrv.loop(ctx)

	sSrv.log.Info("server started")

	sSrv.emitEvent(srvStart, "")

	return nil
}

// Add registers service on the server and returns its ID.
func (sSrv *ServiceServer) AddService(
	name string,
	s ServiceRunner,
	stopCh chan struct{},
	startImmediately bool) (uuid.UUID, error) {

	if s == nil {
		return uuid.Nil, errs.ErrNoService
	}

	id := uuid.New()

	if name == "" {
		name = "Service #" + id.String()
	}

	sr := &serviceRecord{
		id:          id,
		name:        name,
		svc:         s,
		state:       SSRegistered,
		lastError:   nil,
		stopChannel: stopCh}

	sSrv.Lock()
	sSrv.services[id] = sr
	sSrv.Unlock()

	sSrv.emitEvent("SVC_ADDED_EVT",
		fmt.Sprintf(
			"{name: \"%s\", id: \"%v\"}",
			sr.name, sr.id))

	sSrv.log.Debugw("new service registered",
		zap.Stringer("svc_id", id),
		zap.String("name", name))

	if startImmediately {
		if err := sSrv.ExecService(id); err != nil {
			return id, err
		}
	}

	return id, nil
}

// ExecService sends meassge to update Service state
// from SSRegistered to SSReady so it could start.
func (sSrv *ServiceServer) ExecService(
	id uuid.UUID) error {

	if !sSrv.IsRunned() {
		return errs.ErrNoService
	}

	sSrv.Lock()
	defer sSrv.Unlock()

	sr, ok := sSrv.services[id]

	if !ok {
		sSrv.log.Errorw("service isn't found",
			zap.Stringer("svc_id", id))

		return fmt.Errorf("couldn't find service # %v", id)
	}

	srSt := sr.getState()
	if srSt != SSRegistered {
		sSrv.log.Errorw("invalid service state for running",
			zap.Stringer("svc_id", id),
			zap.Stringer("state", srSt))

		return fmt.Errorf("invalid %s service state state %s for running",
			id.String(), srSt.String())
	}

	go func() {
		sr.setState(SSReady, nil)

		select {
		case <-sSrv.ctx.Done():
		case sSrv.svcRunCh <- id:
		}
	}()

	return nil
}

// StopService stops service which have non-nil stopChannel.
func (sSrv *ServiceServer) StopService(id uuid.UUID) error {
	if !sSrv.IsRunned() {
		return errs.ErrNotRunned
	}

	sSrv.Lock()
	sr, ok := sSrv.services[id]
	sSrv.Unlock()

	if !ok {
		return fmt.Errorf("couldn't find service %v", id)
	}

	if srSt := sr.getState(); srSt != SSRunning {
		return fmt.Errorf("service # %v isn't running(%s)",
			id, srSt.String())
	}

	if sr.stopChannel != nil {
		go func() {
			sSrv.log.Debugw("stopping service...",
				zap.Stringer("svc_id", id))
			select {
			case <-sSrv.ctx.Done():
			case sr.stopChannel <- struct{}{}:
				sr.setState(SSEnded, nil)
			}
		}()
	}

	return nil
}

// ResumeService continues previously stopped or ended service.
//
// The service should have stopChannel to be resumed.
// Running services could not be resumed.
func (sSrv *ServiceServer) ResumeService(id uuid.UUID) error {
	sSrv.Lock()
	defer sSrv.Unlock()

	sr, ok := sSrv.services[id]
	if !ok {
		return fmt.Errorf("couldn't find service %v", id)
	}

	if sr.stopChannel == nil {
		return fmt.Errorf("service # %v couldn't have stopChannel :"+
			" cannot be stopped/resumed", id)
	}

	if sr.state == SSRunning {
		return fmt.Errorf("service # %v is running. Couldn't resume it", id)
	}

	sr.setState(SSReady, nil)
	sSrv.svcRunCh <- id

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
	id uuid.UUID) (chan bool, error) {

	sSrv.log.Debugw("wait for service",
		zap.Stringer("svc_id", id))

	// wait for a single service
	if id != uuid.Nil {
		sSrv.Lock()
		sr, ok := sSrv.services[id]
		sSrv.Unlock()

		if !ok {
			sSrv.log.Errorw("service isn't found for wait",
				zap.Stringer("svc_id", id))

			return nil, fmt.Errorf("couldn't find service %s", id.String())
		}

		resChan := make(chan bool)

		go sSrv.waitForSvc(ctx, sr, resChan)

		return resChan, nil
	}

	// wait for all running services
	resChan := make(chan bool)

	sSrv.Lock()
	// I'm not sure what buffer take for accumulator channel
	// and I don't think that additional for loop to count them
	// is a good idea. Moreover some of them could finish in
	// between to loops. So I think number of services / 2 is a
	// good buffer size for accumulating channel
	sumChan := make(chan bool, len(sSrv.services)/2)
	srCount := 0
	for _, sr := range sSrv.services {
		if sr.getState() == SSRunning {
			go sSrv.waitForSvc(ctx, sr, sumChan)
			srCount++
		}
	}
	sSrv.Unlock()

	// if all serviced are already completed, just send true into
	if srCount == 0 {
		go func() {
			// send ok or wait for the context's cancel
			select {
			case <-ctx.Done():
			case resChan <- true:
			}
		}()

		return resChan, nil
	}

	// run service waiting summator to accumulate all
	// signals from previously runned waitForSvc s
	go func(cnt int) {
		for cnt > 0 {
			select {
			case <-ctx.Done():
				resChan <- false
				return

			case res := <-sumChan:
				if !res {
					resChan <- res
					return
				}
				cnt--
			}
		}

		// send notification that all services are
		// completed
		close(sumChan)

		resChan <- true

	}(srCount)

	return resChan, nil
}

// waits for a single service complition.
func (sSrv *ServiceServer) waitForSvc(
	ctx context.Context,
	sr *serviceRecord,
	resChan chan bool) {

	var rCh chan bool

	for {
		if srSt := sr.getState(); srSt == SSEnded || srSt == SSFailed {
			rCh = resChan
		}

		select {
		// if interrupted by context, return false
		case <-ctx.Done():
			resChan <- false

			return

		// this case will be blocked until the service isn't
		// finished and rCh becomes non-nil channel (resChan)
		case rCh <- true:

			return

		// do not block if there is nothing to send or receive
		default:
		}
	}
}

// S2Stat consists the Service Server status returned by Stat mehtod.
type Stat struct {
	Name     string
	ID       uuid.UUID
	Active   bool
	Services int
	Splits   map[string]Info
}

// String implements the fmt.Stringer interface and
// returns the string representation of the S2Stat
func (stat Stat) String() string {
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

// Stat returns s2 server statistics.
func (sSrv *ServiceServer) Stat() Stat {
	stat := new(Stat)
	stat.Splits = make(map[string]Info)

	sSrv.Lock()
	defer sSrv.Unlock()

	stat.Active = sSrv.runned
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

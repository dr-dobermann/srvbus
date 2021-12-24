package es_grpc

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/dr-dobermann/srvbus/es"
	"github.com/dr-dobermann/srvbus/internal/errs"
	pb "github.com/dr-dobermann/srvbus/proto/gen/es_proto"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

const (
	srvStart = "ES_GRPC_START_EVT"
	srvEnd   = "ES_GRPC_END_EVT"
)

var UseHostLogger *zap.SugaredLogger

// EvtServer is a gRPC cover for the es.EventServer.
type EvtServer struct {
	sync.Mutex

	pb.UnimplementedEventServiceServer

	srv *es.EventServer
	log *zap.SugaredLogger

	ctx context.Context

	runned bool

	// subsStreams is used to register and stops the subscription streams.
	// the data as foolow [subscriber_id][subs_stream_id]contex_cancelfunction
	subsStreams map[uuid.UUID]map[uuid.UUID]context.CancelFunc
}

// creates new event server
func New(eSrv *es.EventServer, log *zap.SugaredLogger) (*EvtServer, error) {
	if eSrv == nil {
		return nil, errs.ErrGrpcNoHost
	}

	if log == nil {
		log = eSrv.Logger()
	}

	return &EvtServer{
			srv:         eSrv,
			log:         log,
			subsStreams: make(map[uuid.UUID]map[uuid.UUID]context.CancelFunc)},
		nil
}

// runs a gRPC server for Event Server
func (eSrv *EvtServer) Run(
	ctx context.Context,
	host, port string,
	opts ...grpc.ServerOption) error {

	if eSrv.IsRunned() {
		return errs.ErrAlreadyRunned
	}

	l, err := net.Listen("tcp", fmt.Sprintf("%s:%s", host, port))
	if err != nil {
		return fmt.Errorf("couldn't start tcp listener: %v", err)
	}

	grpcServer := grpc.NewServer(opts...)
	pb.RegisterEventServiceServer(grpcServer, eSrv)

	eSrv.Lock()
	eSrv.runned = true
	eSrv.ctx = ctx
	eSrv.Unlock()

	// start delayed context cancel listener to stop
	// grpc server once context cancelled
	time.AfterFunc(time.Second, func() {
		eSrv.log.Debug("server context stopper started...")

		<-ctx.Done()

		// cancel all existed subscription streams
		// context function should be called according to the
		// documentation to free context resources
		for _, ss := range eSrv.subsStreams {
			for _, cancel := range ss {
				cancel()
			}
		}

		eSrv.log.Debug("context cancelled")

		grpcServer.Stop()
	})

	srvDescr := fmt.Sprintf("{name: \"\", id: \"\"}",
		eSrv.srv.Name, eSrv.srv.ID)

	topic := "/server"

	err = eSrv.srv.AddEvent(topic,
		es.MustEvent(es.NewEventWithString(srvStart, srvDescr)),
		eSrv.srv.ID)
	if err != nil {
		eSrv.log.Warnw("couldn't add an event",
			zap.String("topic", topic),
			zap.Error(err))
	}

	// run grpc server
	eSrv.log.Infow("grpc server started",
		zap.String("host", host),
		zap.String("post", port))

	err = grpcServer.Serve(l)
	if err != nil {
		eSrv.log.Warn("grpc Server ended with error: ", err)
	}

	eSrv.Lock()
	eSrv.runned = false
	eSrv.Unlock()

	err = eSrv.srv.AddEvent(topic,
		es.MustEvent(es.NewEventWithString(srvEnd, srvDescr)),
		eSrv.srv.ID)
	if err != nil {
		eSrv.log.Warnw("couldn't add an event",
			zap.String("topic", topic),
			zap.Error(err))
	}

	eSrv.log.Info("grpc server stopped")

	return err
}

// checks if the grpc server is runned
func (eSrv *EvtServer) IsRunned() bool {
	eSrv.Lock()
	defer eSrv.Unlock()

	return eSrv.runned
}

// checks if the topic exists on host server.
func (eSrv *EvtServer) HasTopic(
	ctx context.Context,
	in *pb.TopicRequest) (*pb.OpResponse, error) {

	var err error

	// log results of the function call
	defer func() {
		if err != nil {
			eSrv.log.Warnw("checking topic failed",
				zap.Error(err))

			return
		}

		eSrv.log.Debug("topic checking succes",
			zap.String("topic", in.GetTopic()))
	}()

	if !eSrv.IsRunned() {
		return nil, errs.ErrNotRunned
	}

	srvID, err := eSrv.checkServerID(in.GetServerId())
	if err != nil {
		return nil, fmt.Errorf("invalid host server id: %v", err)
	}

	res := pb.OpResponse{}

	if eSrv.srv.HasTopic(in.GetTopic()) {
		res.ServerId = srvID.String()
		res.Result = pb.OpResponse_OK

		return &res, nil
	}

	return &res,
		fmt.Errorf("topic '%s' isn't found on server %v",
			in.GetTopic(), srvID)
}

// adds a new topic or a whole topic branch to the host server.
func (eSrv *EvtServer) AddTopics(
	ctx context.Context,
	in *pb.AddTopicReq) (*pb.OpResponse, error) {

	var err error

	// log results of the function call
	defer func() {
		if err != nil {
			eSrv.log.Warnw("topic adding failed",
				zap.String("topic", in.GetTopic()),
				zap.String("add_from", in.GetFromTopic()),
				zap.Error(err))

			return
		}

		eSrv.log.Debug("topic added succesfully",
			zap.String("topic", in.GetTopic()),
			zap.String("add_from", in.GetFromTopic()))
	}()

	if !eSrv.IsRunned() {
		return nil, errs.ErrNotRunned
	}

	srvID, err := eSrv.checkServerID(in.GetServerId())
	if err != nil {
		return nil, fmt.Errorf("invalid host server id: %v", err)
	}

	err = eSrv.srv.AddTopic(in.GetTopic(), in.GetFromTopic())
	if err != nil {
		return nil,
			fmt.Errorf("couldn't add topic '%s' on '%s'",
				in.GetTopic(), in.GetFromTopic())
	}

	return &pb.OpResponse{
			ServerId: srvID.String(),
			Result:   pb.OpResponse_OK},
		nil
}

// check serverID gotten from request
func (eSrv *EvtServer) checkServerID(id string) (uuid.UUID, error) {
	srvID, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid server ID: %v", err)
	}

	if eSrv.srv.ID != srvID {
		return uuid.Nil,
			fmt.Errorf("server ID don't match. Want: %v, got: %v",
				eSrv.srv.ID, srvID)
	}

	return srvID, nil
}

// Returns topic or branch from the host server
func (eSrv *EvtServer) DelTopics(
	ctx context.Context,
	in *pb.DelTopicReq) (*pb.OpResponse, error) {

	var err error

	// log results of the function call
	defer func() {
		if err != nil {
			eSrv.log.Warnw("topic deleting failed",
				zap.String("topic", in.GetTopic()),
				zap.Error(err))

			return
		}

		eSrv.log.Debug("topic added succesfully",
			zap.String("topic", in.GetTopic()))
	}()

	if !eSrv.IsRunned() {
		return nil, errs.ErrNotRunned
	}

	srvID, err := eSrv.checkServerID(in.GetServerId())
	if err != nil {
		return nil, fmt.Errorf("invalid server ID: %v", err)
	}

	err = eSrv.srv.RemoveTopic(
		in.GetTopic(),
		in.GetRecursive())

	if err != nil {
		return nil,
			fmt.Errorf("couldn't remove topic %s recursevely (%t): %v",
				in.GetTopic(), in.GetRecursive(), err)
	}

	return &pb.OpResponse{
			ServerId: srvID.String(),
			Result:   pb.OpResponse_OK},
		nil
}

// adds a new event on the host server.
func (eSrv *EvtServer) AddEvent(
	ctx context.Context,
	in *pb.EventRegistration) (*pb.OpResponse, error) {

	var err error

	// log results of the function call
	defer func() {
		if err != nil {
			eSrv.log.Warnw("event adding failed",
				zap.String("topic", in.GetTopic()),
				zap.String("name", in.GetEvent().EvtName),
				zap.String("sender", in.GetSenderId()),
				zap.Error(err))

			return
		}

		eSrv.log.Debug("event added succesfully",
			zap.String("topic", in.GetTopic()),
			zap.String("name", in.GetEvent().EvtName),
			zap.String("sender", in.GetSenderId()))
	}()

	if !eSrv.IsRunned() {
		return nil, errs.ErrNotRunned
	}

	srvID, err := eSrv.checkServerID(in.GetServerId())
	if err != nil {
		return nil, fmt.Errorf("invalid server ID: %v", err)
	}

	senderID, err := uuid.Parse(strings.Trim(in.GetSenderId(), " "))
	if err != nil {
		return nil, fmt.Errorf("invalid sender ID: %v", err)
	}

	evt, err := es.NewEventWithString(
		in.GetEvent().GetEvtName(),
		in.GetEvent().GetEvtDetails())
	if err != nil {
		return nil, fmt.Errorf("couldn't create event: %v", err)
	}

	err = eSrv.srv.AddEvent(in.GetTopic(), evt, senderID)
	if err != nil {
		return nil, fmt.Errorf("couldn't add event: %v", err)
	}

	return &pb.OpResponse{
			ServerId: srvID.String(),
			Result:   pb.OpResponse_OK},
		nil
}

// creates single or multi- subscription on the host server.
func (eSrv *EvtServer) Subscribe(
	in *pb.SubscriptionRequest,
	stream pb.EventService_SubscribeServer) error {

	var err error

	// log results of the function call
	defer func() {
		if err != nil {
			eSrv.log.Warnw("subscription failed",
				zap.String("topic", in.GetSubscriberId()),
				zap.Error(err))

			return
		}

		eSrv.log.Debug("subscription is succesfull",
			zap.String("topic", in.GetSubscriberId()))
	}()

	if !eSrv.IsRunned() {
		return errs.ErrNotRunned
	}

	srvID, err := eSrv.checkServerID(in.GetServerId())
	if err != nil {
		return fmt.Errorf("invalid server ID: %v", err)
	}

	subscriberID, err := uuid.Parse(strings.Trim(in.GetSubscriberId(), " "))
	if err != nil {
		return fmt.Errorf("invalid sender ID: %v", err)
	}

	streamID, err := uuid.Parse(strings.Trim(in.GetSubsStreamId(), " "))
	if err != nil {
		return fmt.Errorf("couldn't get stream ID: %v", err)
	}

	// register events subscritpion stream cancel function
	if !eSrv.checkSubsStream(subscriberID, streamID, true) {
		return fmt.Errorf(
			"stream [%v] already registered for the sibscriber [%v]",
			streamID, subscriberID)
	}

	strCtx, strCancel := context.WithCancel(eSrv.ctx)

	eSrv.Lock()
	if _, ok := eSrv.subsStreams[subscriberID]; !ok {
		eSrv.subsStreams[subscriberID] = make(map[uuid.UUID]context.CancelFunc)
	}
	eSrv.subsStreams[subscriberID][streamID] = strCancel
	eSrv.Unlock()

	// register subscriptions on the host server
	evtChan := make(chan es.EventEnvelope)
	eSrv.subscribe(in, evtChan, subscriberID)

	// start sending events stream from subscriptions
	err = eSrv.sendEvents(strCtx, evtChan, srvID, stream)

	// remove stream cancel function
	eSrv.Lock()

	// cancel context if it still active to free context resources
	if strCtx.Err() == nil {
		strCancel()
	}

	delete(eSrv.subsStreams[subscriberID], streamID)

	close(evtChan)

	if len(eSrv.subsStreams[subscriberID]) == 0 {
		delete(eSrv.subsStreams, subscriberID)
	}

	eSrv.Unlock()

	if err != nil {
		return fmt.Errorf("event streaming error: %v", err)
	}

	return nil
}

// sends channelled events.
func (eSrv *EvtServer) sendEvents(
	ctx context.Context,
	evtChan chan es.EventEnvelope,
	srvID uuid.UUID,
	stream pb.EventService_SubscribeServer) error {

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case ee := <-evtChan:
			evt := ee.What()

			env := pb.EventEnvelope{
				ServerId: srvID.String(),
				Topic:    ee.Topic,
				SenderId: ee.Publisher.String(),
				RegAt:    ee.RegAt.String(),
				Event: &pb.Event{
					EvtName:    evt.Name,
					EvtDetails: string(evt.Data()),
					Timestamp:  evt.At.Unix()}}

			if err := stream.Send(&env); err != nil {
				return fmt.Errorf(
					"couldn't stream event '%s' in topic '%s' from '%s': %v",
					env.Event.EvtName, env.Topic, env.SenderId, err)
			}
		}
	}
}

// registers subscription on the host server.
func (eSrv *EvtServer) subscribe(
	in *pb.SubscriptionRequest,
	evtChan chan es.EventEnvelope,
	subscriberID uuid.UUID) {

	subsCount := 0

	for i, s := range in.GetSubscriptions() {
		var filters []es.Filter

		for _, f := range s.GetFilters() {
			switch f.GetType() {
			case pb.Filter_HAS_NAME:
				filters = append(filters, es.WithName(f.GetValue()))

			case pb.Filter_IN_NAME:
				filters = append(filters, es.WithSubName(f.GetValue()))

			case pb.Filter_IN_DESCR:
				filters = append(filters, es.WithSubstr(f.GetValue()))
			}
		}

		sr := es.SubscrReq{
			Topic:     s.GetTopic(),
			SubCh:     evtChan,
			Recursive: s.GetRecursive(),
			Depth:     uint(s.GetDepth()),
			StartPos:  int(s.GetStartPos()),
			Filters:   filters,
		}

		subsCount++

		err := eSrv.srv.Subscribe(subscriberID, sr)

		if err != nil {
			eSrv.log.Warnw("subscritpion error",
				zap.Int("number", i),
				zap.String("topic", sr.Topic),
				zap.String("subscriber_id", subscriberID.String()),
				zap.Error(err))

			continue
		}

		eSrv.log.Debugw("new subscritpion",
			zap.Int("number", i),
			zap.String("topic", sr.Topic),
			zap.String("subscriber_id", subscriberID.String()))
	}

	eSrv.log.Debugw("subscritpions added",
		zap.Int("total", subsCount),
		zap.String("subscriber_id", subscriberID.String()))
}

// cancels subsciptions for one or many topics on the host server.
func (eSrv *EvtServer) UnSubscribe(
	ctx context.Context,
	in *pb.UnsubsibeRequest) (*pb.OpResponse, error) {

	var err error

	// log results of the function call
	defer func() {
		if err != nil {
			eSrv.log.Warnw("unsubscription failed",
				zap.String("topic", in.GetSubscriberId()),
				zap.Error(err))

			return
		}

		eSrv.log.Debug("unsubscription is succesfull",
			zap.Strings("topic", in.GetTopics()))
	}()

	if !eSrv.IsRunned() {
		return nil, errs.ErrNotRunned
	}

	srvID, err := eSrv.checkServerID(in.GetServerId())
	if err != nil {
		return nil, fmt.Errorf("invalid server ID: %v", err)
	}

	subscriberID, err := uuid.Parse(strings.Trim(in.GetSubscriberId(), " "))
	if err != nil {
		return nil, fmt.Errorf("invalid sender ID: %v", err)
	}

	err = eSrv.srv.UnSubscribe(subscriberID, in.GetTopics()...)
	if err != nil {
		return nil, fmt.Errorf("unsubscription error: %v", err)
	}

	return &pb.OpResponse{
			ServerId: srvID.String(),
			Result:   pb.OpResponse_OK,
		},
		nil
}

// stops event streaming service of subscription events.
// UnSubscribe only stops sending events into the channel, but not
// stops the streaming service.
func (eSrv *EvtServer) StopSubscriptionStream(
	ctx context.Context, in *pb.StopStreamRequest) (*pb.OpResponse, error) {

	var err error

	// log results of the function call
	defer func() {
		if err != nil {
			eSrv.log.Warnw("streaming stopping failed",
				zap.String("subs_stream_id", in.SubsStreamId),
				zap.Error(err))

			return
		}

		eSrv.log.Debug("streaming stopped",
			zap.String("subs_stream_id", in.GetSubsStreamId()))
	}()

	if !eSrv.IsRunned() {
		return nil, errs.ErrNotRunned
	}

	srvID, err := eSrv.checkServerID(in.GetServerId())
	if err != nil {
		return nil, fmt.Errorf("invalid server ID: %v", err)
	}

	subscriberID, err := uuid.Parse(strings.Trim(in.GetSubscriberId(), " "))
	if err != nil {
		return nil, fmt.Errorf("invalid sender ID: %v", err)
	}

	streamID, err := uuid.Parse(strings.Trim(in.GetSubsStreamId(), " "))
	if err != nil {
		return nil, fmt.Errorf("invalid subscription stream id: %v",
			in.SubsStreamId)
	}

	if !eSrv.checkSubsStream(subscriberID, streamID, false) {
		return nil,
			fmt.Errorf("subscription id [%v] isn't found on the server",
				streamID)
	}

	cancel, ok := eSrv.subsStreams[subscriberID][streamID]
	if !ok {
		return nil,
			fmt.Errorf("checkSubsStream error. "+
				"Stream [%v] cancelFunc doesn't existed for subscriber [%b]",
				streamID, subscriberID)
	}

	cancel()

	return &pb.OpResponse{
			ServerId: srvID.String(),
			Result:   pb.OpResponse_OK},
		nil
}

// checks if the subscriber has active subscription streamID.
// if getNew is empty
func (eSrv *EvtServer) checkSubsStream(subscriberID uuid.UUID,
	streamID uuid.UUID, getNew bool) bool {

	eSrv.Lock()
	defer eSrv.Unlock()

	ss, ok := eSrv.subsStreams[subscriberID]
	if !ok {
		if !getNew {
			return false
		}

		ss = make(map[uuid.UUID]context.CancelFunc)
		eSrv.subsStreams[subscriberID] = ss
	}

	_, ok = ss[streamID]
	if !ok && !getNew { // cancelFunc should be registered
		return false
	}

	if ok && getNew { // cancelFunc shouldn't be registered for a new streamID
		return false
	}

	return true
}

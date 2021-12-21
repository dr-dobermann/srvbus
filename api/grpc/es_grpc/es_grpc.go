package es_grpc

import (
	"context"
	"fmt"
	"sync"

	"github.com/dr-dobermann/srvbus/es"
	pb "github.com/dr-dobermann/srvbus/proto/gen/es_proto"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type EvtServer struct {
	sync.Mutex

	pb.UnimplementedEventServiceServer

	srv *es.EventServer
	log *zap.SugaredLogger

	runned bool
}

func (eSrv *EvtServer) IsRunned() bool {
	eSrv.Lock()
	defer eSrv.Unlock()

	return eSrv.runned
}

// checks if the topic exists on host server.
func (eSrv *EvtServer) HasTopic(
	ctx context.Context,
	in *pb.TopicRequest) (*pb.OpResponse, error) {

	srvID, err := eSrv.checkServerID(in.GetServerId())
	if err != nil {
		return nil, fmt.Errorf("invalid host server id:", err)
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

	srvID, err := eSrv.checkServerID(in.GetServerId())
	if err != nil {
		return nil, fmt.Errorf("invalid host server id:", err)
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

	srvID, err := eSrv.checkServerID(in.GetServerId())
	if err != nil {
		return nil, fmt.Errorf("invalid server ID: %v", err)
	}

	if err = eSrv.srv.RemoveTopic(
		in.GetTopic(),
		in.GetRecursive()); err != nil {

		return nil,
			fmt.Errorf("couldn't remove topic %s recursevely (%t): %v",
				in.GetTopic(), in.GetRecursive(), err)
	}

	return &pb.OpResponse{
			ServerId: srvID.String(),
			Result:   pb.OpResponse_OK},
		nil
}

// // adds a new event on the host server.
// AddEvent(context.Context, *EventRegistration) (*OpResponse, error)
// // creates single or multi- subscription on the host server.
// Subscribe(*SubscriptionRequest, EventService_SubscribeServer) error
// // cancels subsciptions for one or many topics on the host server.
// UnSubscribe(context.Context, *UnsubsibeRequest) (*OpResponse, error)

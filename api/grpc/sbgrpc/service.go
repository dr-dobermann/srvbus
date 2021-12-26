package sbgrpc

import (
	"context"
	"fmt"
	"strings"

	"github.com/dr-dobermann/srvbus/internal/errs"
	pb "github.com/dr-dobermann/srvbus/proto/gen/sb_proto"
	"github.com/google/uuid"
)

// GetMessageServer returns an runned gRPC handler for internal
// MessageServer.
func (sb *SrvBus) GetMessageServer(
	ctx context.Context,
	in *pb.ServerRequest) (*pb.ServerResponse, error) {

	// get requested server_id
	srvID, err := uuid.Parse(strings.Trim(in.GetServerId(), " "))
	if err != nil {
		return nil, fmt.Errorf("invalid server_id: %v", err)
	}

	// check if there is a runned gRPC handler for the MessageServer
	mSrv, err := sb.getMsGrpc()
	if err != nil {
		return nil, err
	}

	// check if requested server_id is equal to the MS grpc handler id
	if srvID != uuid.Nil && srvID != mSrv.ID() {
		return nil,
			fmt.Errorf(
				"MessageServer # %v isn't existed on ServiceBus # %v ("+
					"expected %v or empty uuid)",
				srvID, sb.sBus.ID(), mSrv.ID())
	}

	// return gRPC shell info
	r := pb.ServerResponse{
		ServerId: mSrv.ID().String(),
		Host:     mSrv.Host(),
		Port:     int32(mSrv.Port()),
	}

	return &r, nil
}

func (sb *SrvBus) GetEventServer(
	ctx context.Context,
	in *pb.ServerRequest) (*pb.ServerResponse, error) {

	// get requested server_id
	srvID, err := uuid.Parse(strings.Trim(in.GetServerId(), " "))
	if err != nil {
		return nil, fmt.Errorf("invalid server_id: %v", err)
	}

	eSrv, err := sb.getEsGrpc()
	if err != nil {
		return nil, err
	}

	if srvID != uuid.Nil && srvID != eSrv.ID() {
		return nil,
			fmt.Errorf(
				"EventServer # %v isn't existed on ServiceBus # %v ("+
					"expected %v or empty uuid)",
				srvID, sb.sBus.ID(), eSrv.ID())
	}

	r := pb.ServerResponse{
		ServerId: eSrv.ID().String(),
		Host:     eSrv.Host(),
		Port:     int32(eSrv.Port()),
	}

	return &r, nil
}

func (sb *SrvBus) GetServiceServer(
	ctx context.Context,
	in *pb.ServerRequest) (*pb.ServerResponse, error) {

	return nil, errs.ErrNotImplementedYet
}

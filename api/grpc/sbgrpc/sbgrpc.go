package sbgrpc

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/dr-dobermann/srvbus"
	"github.com/dr-dobermann/srvbus/internal/errs"
	pb "github.com/dr-dobermann/srvbus/proto/gen/sb_proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

const (
	sbStart = "SB_GRPC_START_EVT"
	sbEnd   = "SB_GRPC_END_EVT"
)

var UseHostLogger *zap.SugaredLogger

// EvtServer is a gRPC cover for the es.EventServer.
type SrvBus struct {
	sync.Mutex

	pb.UnimplementedSrvBusServer

	sBus *srvbus.ServiceBus
	log  *zap.SugaredLogger

	ctx context.Context

	runned bool
}

// creates new service bus
func New(sb *srvbus.ServiceBus, log *zap.SugaredLogger) (*SrvBus, error) {
	if sb == nil {
		return nil, errs.ErrGrpcNoHost
	}

	if log == nil {
		log = sb.Logger()
	}

	return &SrvBus{
			sBus: sb,
			log:  log},
		nil
}

// runs a gRPC server for Event Server
func (sb *SrvBus) Run(
	ctx context.Context,
	host, port string,
	opts ...grpc.ServerOption) error {

	if sb.IsRunned() {
		return errs.ErrAlreadyRunned
	}

	if !sb.sBus.IsRunned() {
		if err := sb.sBus.Run(ctx); err != nil {
			return fmt.Errorf("couldn't run host srvBus: %v", err)
		}
	}

	l, err := net.Listen("tcp", fmt.Sprintf("%s:%s", host, port))
	if err != nil {
		return fmt.Errorf("couldn't start tcp listener: %v", err)
	}

	grpcServer := grpc.NewServer(opts...)
	pb.RegisterSrvBusServer(grpcServer, sb)

	sb.Lock()
	sb.runned = true
	sb.ctx = ctx
	sb.Unlock()

	// start context cancel listener to stop
	// grpc server once context cancelled
	go func() {
		sb.log.Debug("server context stopper started...")

		<-ctx.Done()

		sb.log.Debug("context cancelled")

		grpcServer.Stop()
	}()

	sbDescr := fmt.Sprintf("{id: \"%v\"}", sb.sBus.ID())

	sb.sBus.EmitEvent(sbStart, sbDescr)

	// run grpc server
	sb.log.Infow("grpc server started",
		zap.String("host", host),
		zap.String("post", port))

	err = grpcServer.Serve(l)
	if err != nil {
		sb.log.Warn("grpc server ended with error: ", err)
	}

	sb.Lock()
	sb.runned = false
	sb.Unlock()

	sb.sBus.EmitEvent(sbEnd, sbDescr)

	sb.log.Info("grpc server stopped")

	return err
}

// checks if the grpc server is runned
func (sb *SrvBus) IsRunned() bool {
	sb.Lock()
	defer sb.Unlock()

	return sb.runned
}

// GetMessageServer(ctx context.Context, in *ServerRequest, opts ...grpc.CallOption) (*ServerResponse, error)
// GetEventServer(ctx context.Context, in *ServerRequest, opts ...grpc.CallOption) (*ServerResponse, error)
// GetServiceServer(ctx context.Context, in *ServerRequest, opts ...grpc.CallOption) (*ServerResponse, error)

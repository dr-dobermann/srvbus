package sbgrpc

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/dr-dobermann/srvbus"
	"github.com/dr-dobermann/srvbus/api/grpc/msgrpc"
	"github.com/dr-dobermann/srvbus/internal/errs"
	pb "github.com/dr-dobermann/srvbus/proto/gen/sb_proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

const (
	sbStart = "SB_GRPC_START_EVT"
	sbEnd   = "SB_GRPC_END_EVT"

	posEventServer   = 0
	posMessageServer = 1
	posServiceServer = 2
)

var (
	UseHostLogger *zap.SugaredLogger

	srvDefaultPorts map[int]int = map[int]int{
		posEventServer:   50071, // event server
		posMessageServer: 50072, // message server
		posServiceServer: 50073, // service server
	}
)

// EvtServer is a gRPC cover for the es.EventServer.
type SrvBus struct {
	sync.Mutex

	pb.UnimplementedSrvBusServer

	sBus *srvbus.ServiceBus
	log  *zap.SugaredLogger

	ctx context.Context

	host string
	port int

	srvPorts [3]int

	mSrv *msgrpc.MsgServer
	//eSrv *esgrpc.EvtServer

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
			log:  log.Named("GRPC")},
		nil
}

// runs a gRPC server for ServiceBus.
//
// srvPort - is a port numbers for serviceBus gRPC servers
// 		0 -- event server
// 		1 -- message server
// 		2 -- service server
// if any port is 0, then default ports will be used
// [50071, 50072, 50073] accordingly.
func (sb *SrvBus) Run(
	ctx context.Context,
	host string,
	port int,
	srvPorts [3]int,
	opts ...grpc.ServerOption) error {

	if sb.IsRunned() {
		return errs.ErrAlreadyRunned
	}

	if !sb.sBus.IsRunned() {
		if err := sb.sBus.Run(ctx); err != nil {
			return fmt.Errorf("couldn't run host srvBus: %v", err)
		}
	}

	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return fmt.Errorf("couldn't start tcp listener: %v", err)
	}

	grpcServer := grpc.NewServer(opts...)
	pb.RegisterSrvBusServer(grpcServer, sb)

	sb.Lock()
	sb.runned = true
	sb.ctx = ctx
	sb.host = host
	sb.port = port

	for i, p := range srvPorts {
		if p <= 0 {
			p = srvDefaultPorts[i]
		}
		sb.srvPorts[i] = p
	}
	sb.Unlock()

	// start context cancel listener to stop
	// grpc server once context cancelled
	go func() {
		sb.log.Debug("server context stopper started...")

		<-ctx.Done()

		sb.log.Debug("context cancelled. Stopping the gRPC server...")

		grpcServer.Stop()
	}()

	sbDescr := fmt.Sprintf("{id: \"%v\"}", sb.sBus.ID())

	sb.sBus.EmitEvent(sbStart, sbDescr)

	// run grpc server
	sb.log.Infow("grpc server started",
		zap.String("host", host),
		zap.Int("post", port))

	err = grpcServer.Serve(l)
	if err != nil {
		sb.log.Info("grpc server ended with error: ", err)
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

// returns a MessageServer gRPC handler.
//
// if there is no runned handler, it will be created
// and runned.
func (sb *SrvBus) getMsGrpc() (*msgrpc.MsgServer, error) {
	if !sb.IsRunned() {
		return nil, errs.ErrNotRunned
	}

	// if MessageServer's gRPC handler isn't existed yet,
	// then create a new one
	if sb.mSrv == nil {
		var err error

		// take an internal MessageServer from the
		// ServiceBus
		mSrv, err := sb.sBus.GetMessageServer()
		if err != nil {
			return nil,
				fmt.Errorf(
					"couldn't get an MessageServer from ServiceBus: %v", err)
		}

		// and create gRPC hanler for it
		sb.mSrv, err = msgrpc.New(mSrv, nil)
		if err != nil {
			return nil,
				fmt.Errorf(
					"couldn't create an MessageServer gRPC hadler: %v", err)
		}
	}

	// run MS gRPC handler if it's not running yet
	if !sb.mSrv.IsRunned() {
		go func() {
			err := sb.mSrv.Run(sb.ctx, sb.host, sb.srvPorts[posMessageServer])
			if err != nil {
				sb.log.Debug(
					"MessageServer gRPC handler ends with error",
					zap.Error(err))
			}
		}()
	}

	return sb.mSrv, nil
}

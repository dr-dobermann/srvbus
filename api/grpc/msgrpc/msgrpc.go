package msgrpc

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/dr-dobermann/srvbus/internal/errs"
	"github.com/dr-dobermann/srvbus/ms"
	pb "github.com/dr-dobermann/srvbus/proto/gen/ms_proto"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

const (
	srvStart = "MS_GRPC_START_EVT"
	srvEnd   = "MS_GRPC_END_EVT"
)

type MsgServer struct {
	sync.Mutex

	pb.UnimplementedMessengerServer

	log *zap.SugaredLogger
	srv *ms.MessageServer

	runned bool
}

// creates a new GRPC server for the single Message Server.
func New(mSrv *ms.MessageServer,
	log *zap.SugaredLogger) (*MsgServer, error) {

	if mSrv == nil {
		return nil, errs.ErrGrpcNoHost
	}

	if log == nil {
		log = mSrv.Logger()
	}

	ms := &MsgServer{
		log: log.Named("GRPC"),
		srv: mSrv}

	return ms, nil
}

// returns server's running status
func (mSrv *MsgServer) IsRunned() bool {
	mSrv.Lock()
	defer mSrv.Unlock()

	return mSrv.runned
}

// sends a single message or a bunch of them to a server.
// if there is no error, the rpc returns the number of messages sent to
// the queue
func (mSrv *MsgServer) SendMessages(
	ctx context.Context,
	in *pb.SendMsgRequest) (*pb.SendMsgResponse, error) {

	var err error

	res := pb.SendMsgResponse{
		ServerID:   uuid.Nil.String(),
		SentMsgPos: []int32{},
		SentMsgID:  []string{},
	}

	msgs := in.GetMsgs()

	// log results of the function call
	defer func() {
		if err != nil {
			mSrv.log.Warnw("message sending failed",
				zap.Error(err))

			return
		}

		mSrv.log.Debug("message sending succes",
			zap.String("sender", in.GetSenderID()),
			zap.String("queue", in.GetQueue()),
			zap.Int("sent_msg_count", len((res.SentMsgID))),
			zap.Int("asked_msg_count", len(msgs)))
	}()

	if mSrv.srv == nil {
		err = errs.ErrGrpcNoHost

		return nil, err
	}

	// check request
	srvID, err := mSrv.checkServerID(strings.Trim(in.GetServerID(), " "))
	if err != nil {
		err = fmt.Errorf("invalid serverID: %v", err)

		return nil, err
	}

	res.ServerID = srvID.String()

	queue := strings.Trim(in.GetQueue(), " ")
	if len(queue) == 0 {
		err = errs.ErrEmptyQueueName

		return nil, err
	}

	sender, err := uuid.Parse(in.GetSenderID())
	if err != nil {
		err = errs.ErrNoSender

		return nil, err
	}

	// send messages
	for pos, m := range msgs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		default:
		}

		msgID := uuid.New()
		msg, err := ms.NewMsg(msgID, m.Name, bytes.NewBufferString(m.Data))
		if err != nil {
			mSrv.log.Warnw("couldn't create message",
				zap.String("name", m.Name),
				zap.Error(err))
			continue
		}

		err = mSrv.srv.PutMessages(sender, queue, msg)
		if err != nil {
			mSrv.log.Warnw("couldn't send message",
				zap.String("name", m.Name),
				zap.Error(err))
		}

		res.SentMsgPos = append(res.SentMsgPos, int32(pos))
		res.SentMsgID = append(res.SentMsgID, msgID.String())
	}

	return &res, nil
}

// gets a stream of messages from the server.
// returned stream could be empty if there is no messages in the queue
// or there is no _new_ messages for the particular receiverID.
// if you need to read all messages in the queue, just
// set fromBegin to true.
func (mSrv *MsgServer) GetMessages(ctx context.Context,
	in *pb.MessagesRequest) (*pb.MessagesResponse, error) {

	var err error

	msgs := pb.MessagesResponse{
		ServerID: uuid.Nil.String(),
		Messages: []*pb.MessageEnvelope{},
	}

	// log results of the function call
	defer func() {
		if err != nil {
			mSrv.log.Warnw("message getting failed",
				zap.Error(err))

			return
		}

		mSrv.log.Debug("message getting succes",
			zap.String("receiver", in.GetReceiverID()),
			zap.String("queue", in.GetQueue()),
			zap.Int("sent_get_count", len(msgs.Messages)))
	}()

	// check request
	if mSrv.srv == nil {
		err = errs.ErrGrpcNoHost

		return nil, err
	}

	srvID, err := mSrv.checkServerID(strings.Trim(in.GetServerID(), " "))
	if err != nil {
		err = fmt.Errorf("invalid serverID: %v", err)

		return nil, err
	}

	msgs.ServerID = srvID.String()

	receiver, err := uuid.Parse(in.GetReceiverID())
	if err != nil {
		err = fmt.Errorf("receiver is not set: %v", err)

		return nil, err
	}

	queue := strings.Trim(in.GetQueue(), " ")
	if len(queue) == 0 {
		err = errs.ErrEmptyQueueName

		return nil, err
	}

	// read messages
	resChan, err := mSrv.srv.GetMessages(receiver, queue, in.GetFromBegin())
	if err != nil {
		err = fmt.Errorf("couldn't get messages: %v", err)

		return nil, err
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case me, ok := <-resChan:
			if !ok {
				return &msgs, nil
			}

			msgs.Messages = append(msgs.Messages, &pb.MessageEnvelope{
				ServerID: srvID.String(),
				Queue:    queue,
				MsgID:    me.ID.String(),
				At:       me.Registered.String(),
				SenderID: me.Sender.String(),
				Msg: &pb.Message{
					Name: me.Name,
					Data: string(me.Data()),
				},
			})
		}

	}
}

// checks if there is particular queue on the MessageServer.
func (mSrv *MsgServer) HasQueue(
	ctx context.Context,
	in *pb.QueueCheck) (*pb.QueueCheckResponse, error) {

	var err error

	resp := pb.QueueCheckResponse{
		ServerID: uuid.Nil.String(),
		State:    false,
	}

	// log results of the function call
	defer func() {
		if err != nil {
			mSrv.log.Warnw("queue check failed",
				zap.Error(err))

			return
		}

		mSrv.log.Debug("message queue checking succes",
			zap.String("queue", in.GetName()),
			zap.Bool("status", resp.State))
	}()

	// check request
	if mSrv.srv == nil {
		err = errs.ErrGrpcNoHost

		return nil, err
	}

	srvID, err := mSrv.checkServerID(strings.Trim(in.GetServerID(), " "))
	if err != nil {
		err = fmt.Errorf("invalid serverID: %v", err)

		return nil, err
	}

	resp.ServerID = srvID.String()

	queue := strings.Trim(in.GetName(), " ")
	if len(queue) == 0 {
		err = errs.ErrEmptyQueueName

		return nil, err
	}

	// check queue presence
	resp.State = mSrv.srv.HasQueue(queue)

	return &resp, nil
}

// check serverID gotten from request
func (mSrv *MsgServer) checkServerID(id string) (uuid.UUID, error) {
	srvID, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid server ID: %v", err)
	}

	if mSrv.srv.ID() != srvID {
		return uuid.Nil,
			fmt.Errorf("server ID don't match. Want: %v, got: %v",
				mSrv.srv.ID(), srvID)
	}

	return srvID, nil
}

// Runs creates grpc host and runs it.
func (mSrv *MsgServer) Run(
	ctx context.Context,
	host, port string,
	opts ...grpc.ServerOption) error {

	if mSrv.IsRunned() {
		return errs.ErrAlreadyRunned
	}

	// open listener
	l, err := net.Listen("tcp", fmt.Sprintf("%s:%s", host, port))
	if err != nil {
		return fmt.Errorf("couldn't start listener: %v", err)
	}

	// create grpc server with give grpc.Options
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterMessengerServer(grpcServer, mSrv)

	mSrv.Lock()
	mSrv.runned = true
	mSrv.Unlock()

	mSrv.srv.EmitEvent(srvStart,
		fmt.Sprintf(
			"{type: \"grpc\", host: \"%s\", port: %s}",
			host, port))

	mSrv.log.Infow("grpc server started",
		zap.String("host", host),
		zap.String("post", port))

	// start delayed context cancel listener to stop
	// grpc server once context cancelled
	time.AfterFunc(time.Second, func() {
		mSrv.log.Debug("server context stopper started...")
		<-ctx.Done()
		grpcServer.Stop()
	})

	// run grpc server
	err = grpcServer.Serve(l)
	if err != nil {
		mSrv.log.Warnw("grpc Server ended with error: ", zap.Error(err))
	}

	mSrv.Lock()
	mSrv.runned = false
	mSrv.Unlock()

	mSrv.log.Infow("grpc server stopped")

	mSrv.srv.EmitEvent(srvEnd, "")

	return err
}

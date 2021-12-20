package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	pb "github.com/dr-dobermann/srvbus/proto/gen/ms_proto"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var (
	host  = flag.String("host", "localhost", "grpc host")
	port  = flag.Int("port", 50051, "grpc port")
	srvID = flag.String("srv_id", "", "message server ID")
)

func main() {

	flag.Parse()

	lg, err := zap.NewProduction()
	if err != nil {
		panic("coudln't get logger: " + err.Error())
	}
	log := (lg.Sugar())

	log.Infow("trying to connect",
		zap.String("host", *host),
		zap.Int("port", *port))

	conn, err := grpc.Dial(
		fmt.Sprintf("%s:%d", *host, *port),
		grpc.WithInsecure())
	if err != nil {
		log.Fatal("couldn't open gprc connection:", err)
	}

	defer conn.Close()

	c := pb.NewMessengerClient(conn)

	log.Infow("connection established", zap.String("serverID", *srvID))

	sender := uuid.New()

	for i := 1; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)

		r, err := c.SendMessages(ctx, &pb.SendMsgRequest{
			ServerID: *srvID,
			Queue:    "Test_Queue",
			SenderID: sender.String(),
			Msgs: []*pb.Message{
				{Name: fmt.Sprintf("Test_%d", i),
					Data: fmt.Sprintf("Data for msg: Test_%d", i)},
			},
		})
		if err != nil {
			log.Fatal("coudldn't send message", i, ":", err)
		}

		log.Info("message", i, "sucessfully sent as #", r.GetSentMsgID()[0])

		cancel()
	}
}

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/dr-dobermann/srvbus/es"
	pb "github.com/dr-dobermann/srvbus/proto/gen/es_proto"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

var (
	host  = flag.String("host", "localhost", "event server grpc host")
	port  = flag.Int("port", 50051, "event server grpc port")
	srvID = flag.String("evt_srv_ID", "00000000-0000-0000-0000-000000000000", "event server ID")

	debug = flag.Bool("debug", false, "run with debug output")
)

func main() {
	opts := []grpc.DialOption{grpc.WithInsecure()}

	const (
		topic   = "/test_topic"
		evtName = "TEST_EVT"
	)

	flag.Parse()

	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", *host, *port), opts...)
	if err != nil {
		panic("couldn't dial an grpc server: " + err.Error())
	}

	defer conn.Close()

	client := pb.NewEventServiceClient(conn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subscriberID := uuid.New()

	streamID := uuid.New()

	events, err := client.Subscribe(ctx, &pb.SubscriptionRequest{
		ServerId:     *srvID,
		SubscriberId: subscriberID.String(),
		Subscriptions: []*pb.Subscription{
			{
				Topic:     topic,
				Recursive: false,
				Depth:     0,
				StartPos:  int32(es.FromBegin),
				Filters: []*pb.Filter{
					{Value: evtName, Type: pb.Filter_HAS_NAME}},
			},
		},
		SubsStreamId: streamID.String(),
	})
	if err != nil {
		fmt.Println("couldn't open an stream:", err)
		return
	}

	time.AfterFunc(5*time.Second, func() {
		_, err := client.StopSubscriptionStream(ctx, &pb.StopStreamRequest{
			ServerId:     *srvID,
			SubscriberId: subscriberID.String(),
			SubsStreamId: streamID.String(),
		})

		if err != nil {
			fmt.Println("error while stopping stream:", err)
			return
		}

		fmt.Println("event streamer stopped")
	})

	for {
		evtEnv, err := events.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			fmt.Println("stream reading error:", err)
			return
		}

		fmt.Printf("Got event: %s\n  From: %s\n  At: %s\n"+
			"  Topic: %s\n  Details: %s\n",
			evtEnv.GetEvent().GetEvtName(),
			evtEnv.GetSenderId(),
			evtEnv.GetRegAt(),
			evtEnv.GetTopic(),
			evtEnv.GetEvent().GetEvtDetails())
	}
}

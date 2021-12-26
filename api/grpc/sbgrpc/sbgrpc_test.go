package sbgrpc

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dr-dobermann/srvbus"
	"github.com/google/uuid"
	"github.com/matryer/is"
	"go.uber.org/zap"
)

func TestGRPCStart(t *testing.T) {
	is := is.New(t)

	// create logger
	log, err := zap.NewDevelopment()
	is.NoErr(err)
	is.True(log != nil)

	// create service bus
	sb, err := srvbus.New(uuid.New(), log.Sugar())
	is.NoErr(err)
	is.True(sb != nil)

	// create grpc shell for service bus
	sbGrpc, err := New(sb, nil)
	is.NoErr(err)
	is.True(sbGrpc != nil)
	is.True(!sb.IsRunned())

	// start grpc server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := sbGrpc.Run(ctx, "localhost", 50051, [3]int{0, 0, 0})
		if err != nil {
			fmt.Println("service bus grpc handler exit error:", err)
		}
	}()

	retries := 2
	for retries > 0 && !sbGrpc.IsRunned() {
		select {
		case <-ctx.Done():
			fmt.Println("context cancelled")
			retries = 0

		case <-time.After(time.Second):
			retries--
			fmt.Println("tick...")
		}
	}

	is.True(sbGrpc.IsRunned())

	mSrv, err := sbGrpc.getMsGrpc()
	is.NoErr(err)
	is.True(mSrv != nil)

	time.Sleep(10 * time.Second)

	if !mSrv.IsRunned() {
		t.Fatal("MessageServer gRPC hadler is stopped")
	}
}

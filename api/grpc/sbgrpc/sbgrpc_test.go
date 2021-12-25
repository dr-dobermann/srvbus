package sbgrpc

import (
	"context"
	"fmt"
	"sync"
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
	sbGrpc, err := New(sb, log.Sugar())
	is.NoErr(err)
	is.True(sbGrpc != nil)
	is.True(!sb.IsRunned())

	// start grpc server
	ctx, cancel := context.WithDeadline(
		context.Background(),
		time.Now().Add(5*time.Second))
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err := sbGrpc.Run(ctx, "localhost", "500051")
		fmt.Println("grpc service exit error:", err)
		wg.Done()
	}()

	is.True(sbGrpc.IsRunned())

	wg.Wait()
}

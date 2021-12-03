package es

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/matryer/is"
	"go.uber.org/zap"
)

func TestEvntServer(t *testing.T) {
	is := is.New(t)

	log, err := zap.NewDevelopment()
	is.NoErr(err)

	eSrv, err := New(uuid.Nil, "EventServer", log.Sugar())
	is.NoErr(err)
	is.True(eSrv != nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = eSrv.Run(ctx)
	is.NoErr(err)
}

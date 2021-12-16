package srvbus

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/matryer/is"
	"go.uber.org/zap"
)

func TestSBStart(t *testing.T) {
	is := is.New(t)

	// check logger creation
	_, err := New(uuid.Nil, nil)
	is.NoErr(err)

	// create service bus with dev logger
	log, err := zap.NewDevelopment()
	is.NoErr(err)

	sb, err := New(uuid.Nil, log.Sugar())
	is.NoErr(err)
	is.True(sb != nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// run srvbus
	is.NoErr(sb.Run(ctx))
	is.True(sb.IsRunned())
}

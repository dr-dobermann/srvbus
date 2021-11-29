package s2

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dr-dobermann/srvbus/ms"
	"github.com/google/uuid"
	"github.com/matryer/is"
	"go.uber.org/zap"
)

func TestOutputSvc(t *testing.T) {
	is := is.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create service
	out := bytes.NewBuffer([]byte{})
	testStr := []string{"Hello ", "Dober!"}

	svc, err := newOutputService(ctx, out, testStr[0], testStr[1])
	is.NoErr(err)
	is.True(svc != nil)

	// testing invalid params
	t.Run("invalid_params", func(t *testing.T) {
		_, err := newOutputService(ctx, nil, "this is a test")
		is.True(err != nil)
	})

	// run service and check results
	err = svc.Run(ctx)
	is.NoErr(err)
	is.Equal(out.String(), strings.Join(testStr, ""))
}

func TestPutMessagesSvc(t *testing.T) {
	is := is.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log, err := zap.NewDevelopment()
	is.NoErr(err)

	mSrv, err := ms.New(
		uuid.New(),
		"Test MsgSrv",
		log.Sugar())
	is.NoErr(err)
	is.True(mSrv != nil)

	mSrv.Run(ctx)

	svc, err := newPutMessagesService(
		ctx,
		mSrv,
		"test_queue",
		uuid.New(),
		ms.GetMsg(uuid.Nil, "Hello", bytes.NewBufferString("Dober!")))
	is.NoErr(err)
	is.True(svc != nil)

	// err = svc.Run(ctx)
	// is.NoErr(err)

	time.Sleep(5 * time.Second)
}

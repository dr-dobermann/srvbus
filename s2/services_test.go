package s2

import (
	"bytes"
	"context"
	"strings"
	"testing"

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

func TestPutGetMessagesSvc(t *testing.T) {
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

	qn := "test_queue"

	// invalid params for put messages
	t.Run("invalid_params:putMessages", func(t *testing.T) {
		// no message server
		_, err = newPutMessagesService(ctx, nil, "queue", uuid.New(),
			ms.GetMsg(uuid.Nil, "Hello", bytes.NewBufferString("Dober!")))
		is.True(err != nil)

		// no queue
		_, err := newPutMessagesService(ctx, mSrv, "", uuid.New(),
			ms.GetMsg(uuid.Nil, "Hello", bytes.NewBufferString("Dober!")))
		is.True(err != nil)

		// no sender
		_, err = newPutMessagesService(ctx, mSrv, qn, uuid.Nil,
			ms.GetMsg(uuid.Nil, "Hello", bytes.NewBufferString("Dober!")))
		is.True(err != nil)

		// no messages
		_, err = newPutMessagesService(ctx, mSrv, qn, uuid.New(),
			[]*ms.Message{}...)
		is.True(err != nil)
	})

	mSrv.Run(ctx)

	svcPut, err := newPutMessagesService(
		ctx,
		mSrv,
		qn,
		uuid.New(),
		ms.GetMsg(uuid.Nil, "Hello", bytes.NewBufferString("Dober!")))
	is.NoErr(err)
	is.True(svcPut != nil)

	err = svcPut.Run(ctx)
	is.NoErr(err)

	msgChan := make(chan ms.MessageEnvelope)
	svcGet, err := newGetMessagesService(
		ctx,
		mSrv,
		qn,
		true,
		0,
		msgChan)
	is.NoErr(err)
	is.True(svcGet != nil)

}

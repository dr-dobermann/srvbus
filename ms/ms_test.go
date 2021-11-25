package ms

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/matryer/is"
	"go.uber.org/zap"
)

func TestMSrv(t *testing.T) {
	is := is.New(t)

	log, err := zap.NewDevelopment()
	is.NoErr(err)

	sugar := log.Sugar()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const msn = "test_mserver"
	ms, err := New(ctx, uuid.Nil, msn, sugar)
	is.NoErr(err)
	is.True(ms != nil)

	qm := "test_queue"
	mm := []struct{ key, val string }{
		{key: "key1", val: "Hello Dober!"},
		{key: "key2", val: "Hello again Dober!"}}

	err = ms.PutMessages(
		uuid.New(),
		qm,
		GetMsg(uuid.Nil, mm[0].key, bytes.NewBufferString(mm[0].val)),
		GetMsg(uuid.Nil, mm[1].key, bytes.NewBufferString(mm[0].val)))

	is.NoErr(err)
}

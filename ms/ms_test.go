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
	ms, err := New(uuid.Nil, msn, sugar)
	is.NoErr(err)
	is.True(ms != nil)

	ms.Run(ctx)

	qn := "test_queue"
	mm := []struct{ key, val string }{
		{key: "key1", val: "Hello Dober!"},
		{key: "key2", val: "Hello again Dober!"}}

	// put messages to the server
	err = ms.PutMessages(
		uuid.New(),
		qn,
		func() (msgs []*Message) {
			for _, m := range mm {
				msgs = append(msgs,
					GetMsg(
						uuid.Nil,
						m.key,
						bytes.NewBufferString(m.val)))
			}

			return
		}()...)
	is.NoErr(err)

	// get messages from the non-existing queue
	mes, err := ms.GetMessages(uuid.New(), "non-existed_queue", false)
	is.True(err != nil)
	is.True(mes == nil)

	// get messages from the server
	mes, err = ms.GetMessages(uuid.New(), qn, false)
	is.NoErr(err)
	is.Equal(len(mes), 2)

	for i, m := range mes {
		is.Equal(mm[i].key, m.Key)
		is.True(bytes.Equal([]byte(mm[i].val), m.Data()))
	}

	// get queue stats
	qs := ms.Queues()
	is.Equal(len(qs), 1)
	is.Equal(qs[0].Name, qn)
	is.Equal(qs[0].MCount, len(mm))
}

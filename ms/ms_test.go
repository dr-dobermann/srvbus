package ms

import (
	"bytes"
	"context"
	"testing"
	"time"

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
	mm := []struct{ name, val string }{
		{name: "key1", val: "Hello Dober!"},
		{name: "key2", val: "Hello again Dober!"}}

	// put messages to the server
	err = ms.PutMessages(
		uuid.New(),
		qn,
		func() []*Message {
			msgs := []*Message{}
			for _, m := range mm {
				msgs = append(msgs,
					MustMsg(
						NewMsg(
							uuid.Nil,
							m.name,
							bytes.NewBufferString(m.val))))
			}

			return msgs
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
		is.Equal(mm[i].name, m.Name)
		is.True(bytes.Equal([]byte(mm[i].val), m.Data()))
	}

	t.Run("wait_for_queue testing", func(t *testing.T) {
		// invalid queue name
		ch, err := ms.WaitForQueue(ctx, "")
		is.True(ch == nil && err != nil)

		// existed queue name
		ch, err = ms.WaitForQueue(ctx, qn)
		is.True(ch != nil && err == nil)
		is.True(<-ch == true)
		close(ch)

		// non-existed queue name
		qn1 := "non-existed queue"
		wCtx, wCancel := context.WithDeadline(ctx,
			time.Now().Add(2*time.Second))
		defer wCancel()
		ch, err = ms.WaitForQueue(
			wCtx,
			qn1)

		is.True(ch != nil && err == nil)
		is.True(!<-ch)
		close(ch)

		// fmt.Println("---> waiting result :", found)
		is.True(!ms.HasQueue(qn1))
	})

	// get queue stats
	qs := ms.Queues()
	is.Equal(len(qs), 1)
	is.Equal(qs[0].Name, qn)
	is.Equal(qs[0].MCount, len(mm))
}

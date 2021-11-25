package ms

import (
	"bytes"
	"testing"

	"github.com/google/uuid"
	"github.com/matryer/is"
	"go.uber.org/zap"
)

func TestMsg(t *testing.T) {
	is := is.New(t)

	key := "Greetings"
	value := "Hello Dober!"

	msg, err := NewMsg(uuid.Nil, key, bytes.NewBufferString(value))
	is.NoErr(err)

	is.Equal(string(msg.Data()), value)

	msg2 := msg.Copy()

	var buf bytes.Buffer
	buf.ReadFrom(msg2)

	is.Equal(buf.String(), value)
}

func TestQueue(t *testing.T) {
	is := is.New(t)

	qn := "test_queue"
	mm := []struct{ key, value string }{
		{"msg1", "hello Dober!"},
		{"msg2", "hello again, Dober!"}}

	logger, err := zap.NewDevelopment()
	is.NoErr(err)

	defer logger.Sync()

	// queue creating tests
	sugar := logger.Sugar()

	q, err := newQueue(uuid.Nil, qn, sugar)
	is.NoErr(err)
	is.Equal(qn, q.Name())
	is.True(q.ID() != uuid.Nil)

	t.Run("queue with no logger",
		func(t *testing.T) {
			q2, err := newQueue(uuid.Nil, "", nil)
			is.True(err != nil)
			is.Equal(q2, nil)
		})

	t.Run("auto-set of queue name",
		func(t *testing.T) {
			qid := uuid.New()
			q2, err := newQueue(qid, "", sugar)
			is.NoErr(err)
			is.Equal(q2.Name(), "mQueue #"+qid.String())
		})

	// testing of putting messages into the queue
	t.Run("empty msg list",
		func(t *testing.T) {
			if err = <-q.PutMessages(uuid.New(), []*Message{}...); err == nil {
				t.Error("put empty messages list")
			}
		})

	var msgs []*Message
	for _, m := range mm {
		msgs = append(msgs,
			GetMsg(uuid.Nil, m.key, bytes.NewBufferString(m.value)))
	}

	err = <-q.PutMessages(uuid.New(), msgs...)
	is.NoErr(err)

	is.Equal(q.Count(), 2)

	// testing of getting messages from the queue
	t.Run("empty_reciever", func(t *testing.T) {
		_, err := q.GetMessages(uuid.Nil, false)
		is.True(err != nil)
	})

	recieverID := uuid.New()
	mes, err := q.GetMessages(recieverID, false)
	is.NoErr(err)
	is.Equal(len(mes), 2)

	for i, m := range mes {
		is.Equal(m.Key, mm[i].key)
		is.Equal(string(m.Data()), mm[i].value)
	}

	// trying to read new messages
	mes, err = q.GetMessages(recieverID, false)
	is.NoErr(err)
	is.Equal(len(mes), 0)

	// rereading messages from the begin
	mes, err = q.GetMessages(recieverID, true)
	is.NoErr(err)
	is.Equal(len(mes), 2)
}

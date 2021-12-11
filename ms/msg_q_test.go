package ms

import (
	"bytes"
	"testing"

	"github.com/google/uuid"
	"github.com/matryer/is"
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

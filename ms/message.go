package ms

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/dr-dobermann/srvbus/internal/ds"
	"github.com/google/uuid"
)

// Message represent the single message on the server
type Message struct {
	ds.DataItem

	ID uuid.UUID
}

// Copy returns a copy of the whole Message m.
func (m *Message) Copy() *Message {
	nm := new(Message)
	nm.ID = m.ID
	nm.DataItem = *m.DataItem.Copy()

	return nm
}

func MustMsg(msg *Message, err error) *Message {
	if err != nil {
		panic(err.Error())
	}

	return msg
}

// NewMsg creates an Message with Key key and Data data and returns the pointer
// to it.
//
// if the data is more than 8k, then error will be returned.
//
//nolint:revive
func NewMsg(id uuid.UUID, name string, r io.Reader) (*Message, error) {
	if id == uuid.Nil {
		id = uuid.New()
	}

	buf, err := ioutil.ReadAll(r)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("message %s creation : %w", name, err)
	}

	const (
		kBytes        = 1 << 10
		maxMsgDataLen = 8 * kBytes // 8kbytes
	)

	if len(buf) > maxMsgDataLen {
		return nil, fmt.Errorf("message %s is large then 8kbyte(%d) ",
			name, len(buf))
	}

	return &Message{
			DataItem: *ds.New(name, buf),
			ID:       id},
		nil
}

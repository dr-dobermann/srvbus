package ms

import (
	"fmt"
	"io"
	"io/ioutil"
)

// =============================================================================
// Message represent the single message on the server
type Message struct {
	Key    string
	data   []byte
	readed int
}

// Read implements a io.Reader interface for
// m.data.
func (m *Message) Read(p []byte) (n int, err error) {
	// if len(p) == 0 then return 0 and nil according io.Reader description.
	if len(p) == 0 {
		return
	}

	n = copy(p, m.data[m.readed:])

	m.readed += n
	if m.readed == len(m.data) {
		err = io.EOF
	}

	return
}

// Data returns a copy of m.data.
func (m *Message) Data() []byte {
	return append([]byte{}, m.data...)
}

// Copy returns a copy of the whole Message m.
func (m *Message) Copy() *Message {
	return &Message{
		Key:  m.Key,
		data: append([]byte{}, m.data...),
	}
}

// NewMsg creates an Message with Key key and Data data and returns the pointer
// to it.

// if the data is more than 8k, then error will be returned.
func NewMsg(key string, r io.Reader) (*Message, error) {
	buf, err := ioutil.ReadAll(r)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("message %s creation : %w", key, err)
	}

	const maxMsgDataLen = 8 * (1 << 10) // 8kbytes

	if len(buf) > maxMsgDataLen {
		return nil, fmt.Errorf("message %s is too large :%d ", key, len(buf))
	}

	return &Message{Key: key, data: buf}, nil
}

// GetMsg returns a Message or rise panic on error.
func GetMsg(key string, r io.Reader) *Message {
	m, err := NewMsg(key, r)
	if err != nil {
		panic(err.Error())
	}

	return m
}

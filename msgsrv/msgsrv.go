package msgsrv

import (
	"fmt"
	"io"
	"io/ioutil"
	"sync"
	"time"
)

type MessageServerError struct {
	ms  *MessageServer
	msg string
	Err error
}

func (mse MessageServerError) Error() string {
	em := ""
	if mse.ms != nil {
		em += "[" + mse.ms.Name + "] "
	}

	em += em + mse.msg

	if mse.Err != nil {
		em += " : " + mse.Err.Error()
	}

	return em
}

// NewMessageServerError creates one Message Server error
func NewMessageServerError(ms *MessageServer, msg string, err error) error {

	return MessageServerError{ms, msg, err}
}

type Message struct {
	RegTime time.Time
	Key     string
	data    []byte
	readed  int
}

// Read implements a io.Reader interface for
// m.data.
func (m *Message) Read(p []byte) (n int, err error) {
	ln := len(p)
	if ln == 0 {
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

// GetCopy creates and returns a copy of
// the whole Message m.
func (m *Message) GetCopy() *Message {

	nm := &Message{
		RegTime: m.RegTime,
		Key:     m.Key,
		data:    append([]byte{}, m.data...),
	}

	return nm
}

// NewMsg creates an Message with Key key and Data data and returns the pointer
// to it.
// if the data is more than 8k, then error will be returned.
func NewMsg(key string, r io.Reader) (*Message, error) {

	b, err := ioutil.ReadAll(r)
	if err != nil && err != io.EOF {
		return nil, NewMessageServerError(nil,
			"couldn't read data for meddage "+key,
			err)
	}

	if len(b) > 8*(2<<10) {
		return nil,
			NewMessageServerError(nil,
				fmt.Sprintf("message %s is too large :%d ", key, len(b)),
				nil)
	}

	return &Message{Key: key, data: b}, nil
}

// MustGetMsg returns a Message or rise panic on error.
func MustGetMsg(key string, r io.Reader) *Message {
	m, err := NewMsg(key, r)
	if err != nil {
		panic(err.Error())
	}

	return m
}

type MQueue struct {
	sync.Mutex

	name       string
	messages   []*Message
	lastReaded int
}

type MessageServer struct {
	sync.Mutex

	Name   string
	queues map[string]*MQueue
}

// NewMessageServer returns a new MessageServer instance
// named name.
// If name is an empty string, the defult name "Message Server"
// would be given to the new server.
func NewMessageServer(name string) *MessageServer {
	if name == "" {
		name = "Message Server"
	}

	ms := new(MessageServer)
	ms.Name = name
	ms.queues = make(map[string]*MQueue)

	return ms
}

// PutMessages puts a list of messages into queue qname.
// If name of queue is empty, then error will be returned.
// If lenght of msg is 0, then error will be returned.
func (ms *MessageServer) PutMessages(qname string, msg ...Message) error {
	if qname == "" {
		return NewMessageServerError(ms, "queue name is empty", nil)
	}

	ms.Lock()
	q, ok := ms.queues[qname]
	if !ok {
		ms.queues[qname] = &MQueue{
			name:       qname,
			messages:   []*Message{},
			lastReaded: 0}

		q = ms.queues[qname]
	}
	ms.Unlock()

	for _, m := range msg {
		q.Lock()
		m.RegTime = time.Now()
		q.messages = append(q.messages, m.GetCopy())
		q.Unlock()
	}

	return nil
}

func checkQueue(ms *MessageServer, qname string) (*MQueue, error) {
	ms.Lock()
	q, ok := ms.queues[qname]
	ms.Unlock()

	if !ok {
		return nil,
			NewMessageServerError(ms,
				"couldn't find queue "+qname, nil)
	}

	return q, nil
}

// GetMessages returns a list of messages from queue qname.
func (ms *MessageServer) GetMesages(qname string) ([]Message, error) {
	mm := []Message{}

	q, err := checkQueue(ms, qname)
	if err != nil {
		return nil, err
	}

	q.Lock()
	defer q.Unlock()

	for _, m := range q.messages[q.lastReaded:] {
		mm = append(mm, *m.GetCopy())
	}
	q.lastReaded = len(q.messages)

	return mm, nil
}

// ResetQueue resets the current position of the queue qname to start.
// if there is no queue qname on the MessageServer ms, error would be fired.
func (ms *MessageServer) ResetQueue(qname string) error {
	q, err := checkQueue(ms, qname)
	if err != nil {
		return err
	}

	q.Lock()
	q.lastReaded = 0
	q.Unlock()

	return nil
}

// HasQueue checks presence of queue qname on the MessageServer ms.
func (ms *MessageServer) HasQueue(qname string) bool {
	ms.Lock()
	_, ok := ms.queues[qname]
	ms.Unlock()

	return ok
}

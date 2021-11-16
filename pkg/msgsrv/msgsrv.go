package msgsrv

import (
	"fmt"
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
		em += "[" + mse.ms.name + "] "
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
	Data    []byte
}

// Msg creates an Message with Key key and Data data and returns the pointer
// to it.
// if the data is more than 8k, then error will be returned.
func Msg(key string, data []byte) (*Message, error) {
	if len(data) > 8*(2<<10) {
		return nil,
			NewMessageServerError(nil,
				fmt.Sprintf("message %s is too large :%d ", key, len(data)),
				nil)
	}

	return &Message{Key: key, Data: append([]byte{}, data...)}, nil
}

func MustMsg(key string, data []byte) *Message {
	m, err := Msg(key, data)
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
	name   string
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
	ms.name = name
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

	q, ok := ms.queues[qname]
	if !ok {
		ms.queues[qname] = &MQueue{
			name:       qname,
			messages:   []*Message{},
			lastReaded: 0}

		q = ms.queues[qname]
	}

	for _, m := range msg {
		q.Lock()
		q.messages = append(q.messages, &Message{time.Now(), m.Key, m.Data})
		q.Unlock()
	}

	return nil
}

func checkQueue(ms *MessageServer, qname string) (*MQueue, error) {
	q, ok := ms.queues[qname]
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
		mm = append(mm, Message{m.RegTime, m.Key, m.Data})
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
	_, ok := ms.queues[qname]

	return ok
}

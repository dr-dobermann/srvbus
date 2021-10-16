package srvbus

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type Message struct {
	when   time.Time
	msg    string
	readed bool
}

func (m *Message) Read() (string, time.Time) {
	m.readed = true

	return m.msg, m.when
}

func (m *Message) String() string {
	return fmt.Sprintf("when: %v, msg: %v", m.when, m.msg)
}

type MessageServer struct {
	name       string
	m          *sync.Mutex // TODO: consider to create mutexes for every single queue
	qq         map[string][]Message
	minTimeout int64
}

type MessageServerError struct {
	MSrv    *MessageServer
	Message string
}

func (mew MessageServerError) Error() string {
	if mew.MSrv == nil {
		return mew.Message
	}

	return fmt.Sprintf("[%s]: %s", mew.MSrv.name, mew.Message)
}

func NewMessageServerError(ms *MessageServer, msg string) error {
	return MessageServerError{ms, msg}
}

func (s MessageServer) GetNextTime() time.Time {
	return time.Now().Add(time.Duration(s.minTimeout * int64(time.Second)))
}

func NewMessageServer(n string) *MessageServer {
	return &MessageServer{n, new(sync.Mutex), make(map[string][]Message), 1}
}

func (s MessageServer) ListQueues() []string {
	qq := []string{}

	for qn := range s.qq {
		qq = append(qq, qn)
	}

	return qq
}
func (s *MessageServer) AddMessage(q string, m string) {
	s.m.Lock()

	if _, ok := s.qq[q]; !ok {
		s.qq[q] = []Message{}
	}

	log.Print("Message added into queue ", q)

	s.qq[q] = append(s.qq[q], Message{
		when:   time.Now(),
		msg:    m,
		readed: false,
	})

	s.m.Unlock()
}

// GetMessages returns no more than MsgCount unreaded messages
// from the message q.
// If readed is true, it returns also previously readed messages
// If qShouldBe is true, then error returns if there is no queue named q,
// else the empty messages slice will be returned
func (s *MessageServer) GetMessages(q string, readed bool, msgCount int64, qShouldBe bool) ([]Message, error) {
	mm := []Message{}

	s.m.Lock()
	defer s.m.Unlock()

	if _, ok := s.qq[q]; !ok {
		if qShouldBe {
			return nil, NewMessageServerError(s, fmt.Sprintf("Queue \"%v\" is not found", q))
		}

		return mm, nil
	}

	for i, m := range s.qq[q] {
		if m.readed && !readed {
			continue
		}

		s.qq[q][i].readed = true

		mm = append(mm, m)

		if msgCount > 0 {
			msgCount--
			if msgCount == 0 {
				break
			}
		}
	}

	return mm, nil
}

type MsgServerDef struct {
	MsgServer *MessageServer
	QueueName string
	// timeout in seconds between messages request.
	// if 0 then server default timeout used
	Timeout int64
}

const (
	getMsgParams = 2
	putMsgParams = 2
)

// SrvGetMessage gets unread messages from message server.
// It needs two parameters:
//   - MsgServerDef as message server and queue definition
//   - msgCounts(int64) as number of messages to read. If it's value -1
//     then it would be read all the messages until server ends
func SrvGetMessages(ctx context.Context, s *Service) error {
	var (
		mr   MsgServerDef
		cntr int64
		ok   bool
	)
	
	switch {
	case len(s.params) < getMsgParams:
	    return NewServiceError(s,
			fmt.Sprintf("Too few parameter to start SrvGetMessages service %v out of 2",
				len(s.params)))
	case mr, ok = s.params[0].(MsgServerDef); !ok:
	    return NewServiceError(s, "Could't get message server definition")

	case cntr, ok = s.params[1].(int64); !ok:
	    return NewServiceError(s, "Could't get message counter")
	}

	mm, err := mr.MsgServer.GetMessages(mr.QueueName, false, cntr, false)
	if err != nil {
		return err
	}

	log.Print("Got ", len(mm), " messages for service ", s.id)

	for _, m := range mm {
		s.m.Lock()
		s.results = append(s.results, m)
		s.m.Unlock()

		if cntr < 0 {
			continue
		}

		cntr--
		if cntr == 0 {
			s.SetState(SSFinished)

			return nil
		}
	}

	if cntr > 0 {
		log.Print(cntr, " messages left for service ", s.id)
		s.SetState(SSAwaitsResponse)

		if mr.Timeout == 0 {
			s.nextCheck = mr.MsgServer.GetNextTime()
		} else {
			s.nextCheck = time.Now().Add(time.Duration(mr.Timeout * int64(time.Second)))
		}
	} else {
		s.m.Lock()
		s.state = SSFinished
		s.m.Unlock()
	}

	// update counter
	s.params[1] = cntr

	return nil
}

// SrvPutMessages saves messages into specific queue of message server.
// It takes two or more parameters:
//  - MsgServerDef as message server and queue definition
//  - messages in every single parameter
func SrvPutMessages(ctx context.Context, s *Service) error {
	if len(s.params) < putMsgParams {
		return NewServiceError(s,
			fmt.Sprintf("Too few parameter to start SrvPutMessages service %v out of 2",
				len(s.params)))
	}

	var (
		mr MsgServerDef
		ok bool
	)

	if mr, ok = s.params[0].(MsgServerDef); !ok {
		return NewServiceError(s, "Could't get message server definition")
	}

	for i, m := range s.params[1:] {
		msg, ok := m.(string)
		if !ok {
			log.Print("ERROR: couldn't convert ", i, "th parameter to string in AddMessage")

			s.SetState(SSBroken)

			continue
		}

		mr.MsgServer.AddMessage(mr.QueueName, msg)
	}

	if s.state != SSBroken {
		s.SetState(SSFinished)
	}

	return nil
}

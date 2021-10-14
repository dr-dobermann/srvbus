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
	m          *sync.Mutex // TODO: consider to create mutexes for every single queue
	qq         map[string][]Message
	minTimeout int64
}

func (s MessageServer) GetNextTime() time.Time {
	return time.Now().Add(time.Duration(s.minTimeout * int64(time.Second)))
}

func NewMessageServer() *MessageServer {
	return &MessageServer{new(sync.Mutex), make(map[string][]Message), 1}
}

func (s *MessageServer) AddMessage(q string, m string) {
	if _, ok := s.qq[q]; !ok {
		s.qq[q] = make([]Message, 0)
	}
	log.Print("Message added into queue ", q)
	s.m.Lock()
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
func (s *MessageServer) GetMessages(q string, readed bool,
	MsgCount int, qShouldBe bool) ([]Message, error) {
	var mm []Message
	if _, ok := s.qq[q]; !ok {
		if qShouldBe {
			return nil, fmt.Errorf("queue %v couldn't be found on the message server", q)
		}
		return mm, nil
	}
	s.m.Lock()
	for i, m := range s.qq[q] {
		if m.readed && !readed {
			continue
		}
		s.qq[q][i].readed = true
		mm = append(mm, m)
		if MsgCount > 0 {
			MsgCount--
			if MsgCount == 0 {
				break
			}
		}
	}
	s.m.Unlock()

	return mm, nil
}

type MsgServerDef struct {
	MsgServer *MessageServer
	QueueName string
	// timeout in seconds between messages request.
	// if 0 then server default timeout used
	Timeout int64
	// Messages cout to stop requesting
	// if -1 there is no message limit
	MsgCount int
}

func SrvGetMessages(ctx context.Context, s *Service) error {
	var (
		mr MsgServerDef
		ok bool
	)
	if mr, ok = s.params[0].(MsgServerDef); !ok {
		return fmt.Errorf("could't get message server definition for %v service", s.id)
	}

	mm, err := mr.MsgServer.GetMessages(mr.QueueName, false, mr.MsgCount, false)
	if err != nil {
		return err
	}
	log.Print("Got ", len(mm), " messages for service ", s.id)
	for _, m := range mm {
		s.results = append(s.results, m)
		if mr.MsgCount < 0 {
			continue
		}
		mr.MsgCount--
		if mr.MsgCount == 0 {
			s.state = SSFinished
			return nil
		}
	}
	if mr.MsgCount > 0 {
		log.Print(mr.MsgCount, " messages left for service ", s.id)
		s.state = SSAwaitsResponse
		if mr.Timeout == 0 {
			s.nextCheck = mr.MsgServer.GetNextTime()
		} else {
			s.nextCheck = time.Now().Add(time.Duration(mr.Timeout * int64(time.Second)))
		}
	} else {
		s.state = SSFinished
	}
	// update counter
	s.params[0] = mr

	return nil
}

func SrvPutMessages(ctx context.Context, s *Service) error {
	var (
		mr MsgServerDef
		ok bool
	)
	if len(s.params) < 2 {
		return fmt.Errorf("not enough parameters to add message to the server for %v service", s.id)
	}
	if mr, ok = s.params[0].(MsgServerDef); !ok {
		return fmt.Errorf("could't get message server definition for %v service", s.id)
	}
	for i, m := range s.params[1:] {
		msg, ok := m.(string)
		if !ok {
			log.Print("ERROR: couldn't convert ", i, "th parameter to string in AddMessage")
			s.state = SSBroken
			continue
		}
		mr.MsgServer.AddMessage(mr.QueueName, msg)
	}
	if s.state != SSBroken {
		s.state = SSFinished
	}

	return nil
}

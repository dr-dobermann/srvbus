package srvbus

import (
	"context"
	"fmt"
	"log"
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
	qq map[string][]Message
}

func NewMessageServer() *MessageServer {
	return &MessageServer{make(map[string][]Message)}
}

func (s *MessageServer) Add(q string, m string) {
	if _, ok := s.qq[q]; !ok {
		s.qq[q] = make([]Message, 0)
	}
	s.qq[q] = append(s.qq[q], Message{
		when:   time.Now(),
		msg:    m,
		readed: false,
	})
}

func (s *MessageServer) GetMessages(q string, readed bool, cnt int) ([]Message, error) {
	if _, ok := s.qq[q]; !ok {
		return nil, fmt.Errorf("couldn't find queue %v", q)
	}
	var mm []Message
	for i, m := range s.qq[q] {
		if m.readed && !readed {
			continue
		}
		s.qq[q][i].readed = true
		mm = append(mm, m)
		if cnt > 0 {
			cnt--
			if cnt == 0 {
				break
			}
		}
	}
	return mm, nil
}

type msgReaderDef struct {
	msrv *MessageServer
	q    string
	tout int64
	cnt  int
}

func SrvGetMessage(ctx context.Context, s *Service) error {
	if len(s.params) < 4 {
		return fmt.Errorf("There are no enough parameters (%v out of 4) for service %v invocation", len(s.params), s.id)
	}

	var (
		mr msgReaderDef
		ok bool
	)
	if len(s.params) == 5 { // load saved parameters
		if mr, ok = s.params[4].(msgReaderDef); !ok {
			return fmt.Errorf("could't get saved message reader definition for %v service", s.id)
		}
	} else { // parse paramters
		if mr.msrv, ok = s.params[0].(*MessageServer); !ok {
			return fmt.Errorf("couldn't get an MessageServer address for %v service", s.id)
		}
		if mr.q, ok = s.params[1].(string); !ok || len(mr.q) == 0 {
			return fmt.Errorf("couldn't get an queue name for %v service", s.id)
		}
		if mr.tout, ok = s.params[2].(int64); !ok {
			return fmt.Errorf("Could not get an timeout value for %v service", s.id)
		}
		if mr.cnt, ok = s.params[3].(int); !ok {
			return fmt.Errorf("Could not get an message count value for %v service", s.id)
		}
		// save parameters for future usage
		s.params = append(s.params, mr)
	}

	mm, err := mr.msrv.GetMessages(mr.q, false, mr.cnt)
	if err != nil {
		return err
	}
	log.Print("Got ", len(mm), " messages for service ", s.id)
	for _, m := range mm {
		s.results = append(s.results, m)
		if mr.cnt < 0 {
			continue
		}
		mr.cnt--
		if mr.cnt == 0 {
			s.state = SSFinished
			return nil
		}
	}
	if mr.cnt > 0 {
		log.Print(mr.cnt, " messages left for service ", s.id)
		s.state = SSAwaitsResponse
		s.nextCheck = time.Now().Add(time.Duration(mr.tout * int64(time.Second)))
	} else {
		s.state = SSFinished
	}
	s.params[4] = mr

	return nil
}

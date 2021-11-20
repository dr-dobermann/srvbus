package msgsrv

import (
	"strings"
	"testing"
)

func TestMessageServer(t *testing.T) {
	sn := "test_server"
	ms := NewMessageServer(sn)

	if ms == nil {
		t.Fatal("Couldn't create the Message Server")
	}

	qn := "test_queue"
	tm_str := "test_message"
	tm_key, tm_r := "key", strings.NewReader(tm_str)
	if err := ms.PutMessages(qn, *MustGetMsg(tm_key, tm_r)); err != nil {
		t.Fatal("Couldn't put message to Server", err.Error())
	}

	// check putting message into non-named queue
	if err := ms.PutMessages("", *MustGetMsg(tm_key, tm_r)); err == nil {
		t.Fatal("Adding message into empty queue")
	}

	q, ok := ms.queues[qn]
	if !ok {
		t.Fatal("Couldn't find queue", qn)
	}

	if len(q.messages) != 1 {
		t.Fatal("Invalid messages number :", len(q.messages))
	}

	checkData := func(ms *MessageServer) {
		mm, err := ms.GetMesages(qn)
		if err != nil {
			t.Fatal("Couldn't get test messages")
		}
		if len(mm) != 1 {
			t.Fatal("Invalid message number :", len(mm))
		}
		if string(mm[0].Data()) != tm_str {
			t.Fatal("Data error! Expected", string(tm_str), ", got", string(mm[0].Data()))
		}
	}

	checkData(ms)

	// try to read again
	if mm, err := ms.GetMesages(qn); err != nil {
		t.Error("Getting data error", err)
	} else {
		if len(mm) != 0 {
			t.Error("Got extra data :", mm)
		}
	}

	if _, err := ms.GetMesages("non_existed_queue"); err == nil {
		t.Error("Reading form worng queue")
	}

	if err := ms.ResetQueue(qn); err != nil {
		t.Error("Couldn't reset queue", qn, ":", err)
	}

	checkData(ms)

	if ms.HasQueue("unknown_queue") {
		t.Error("Invalid queue name")
	}
}

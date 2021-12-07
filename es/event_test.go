package es

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	"github.com/matryer/is"
)

func TestEvents(t *testing.T) {
	is := is.New(t)

	evt, err := NewEventWithString("Event1", "Description of Event1")
	is.NoErr(err)
	is.True(evt != nil)
	is.Equal(evt.Name, "Event1")
	is.Equal(evt.Data(), []byte("Description of Event1"))

	evt1, err := NewEventWithReader("Event2",
		bytes.NewBufferString("Description of Event2"))
	is.NoErr(err)
	is.True(evt1 != nil)
	is.Equal(evt1.Name, "Event2")

	buf, err := ioutil.ReadAll(evt1)
	if err != nil && err != io.EOF {
		t.Fatal("couldn't read data from Event", err)
	}
	is.Equal(buf, []byte("Description of Event2"))

	t.Run("invalid_params", func(t *testing.T) {
		// empty event name
		_, err := NewEventWithString("", "")
		is.True(err != nil)

		// empty reader
		_, err = NewEventWithReader("no reader", nil)
		is.True(err != nil)

		// empty event name
		_, err = NewEventWithReader("      ", nil)
		is.True(err != nil)
	})

}

package es

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"github.com/dr-dobermann/srvbus/internal/ds"
	"github.com/google/uuid"
)

// =============================================================================
//                                  Event
//
// Event represent the single event in the system which
// occurs somewhere and At given time, has name and other details in data.
type Event struct {
	ds.DataItem
	At time.Time
}

func (e *Event) String() string {
	return fmt.Sprintf("Evt '%s' @ %v [%s]",
		e.Name, e.At, string(e.Data()))
}

// MustEvent checks if there is no error while the Event creation.
// If any, then panic fired.
func MustEvent(evt *Event, err error) *Event {
	if err != nil {
		panic(err.Error())
	}

	return evt
}

// NewEventWithReader creates a new Event from given io.Reader.
func NewEventWithReader(name string, r io.Reader) (*Event, error) {
	name = strings.Trim(name, " ")
	if name == "" {
		return nil, fmt.Errorf("couldn't create an Event with empty name")
	}

	if r == nil {
		return nil, fmt.Errorf("no io.Reader given for Event '%s'", name)
	}

	buf, err := ioutil.ReadAll(r)
	if err != nil {
		if err != io.EOF {
			return nil, err
		}
	}

	return &Event{
			DataItem: *ds.NewItem(name, buf),
			At:       time.Now(),
		},
		nil
}

// NewEventWithReader creates a new Event from given string.
func NewEventWithString(name string, data string) (*Event, error) {
	name = strings.Trim(name, " ")
	if name == "" {
		return nil, fmt.Errorf("couldn't create an Event with empty name")
	}

	return &Event{
			DataItem: *ds.NewItem(name, []byte(data)),
			At:       time.Now(),
		},
		nil
}

// =============================================================================
//                               EventEnvelope
//
// EventEnvelope covers single Event and adds compliment information from its
// registration.
type EventEnvelope struct {
	event *Event

	Topic     string
	Publisher uuid.UUID
	RegAt     time.Time

	// event index in the topic storage
	Index int
}

// checks obligatory EventEnvelope fields
func (ee EventEnvelope) check() error {
	if ee.event == nil {
		return fmt.Errorf("empty event")
	}

	if ee.Publisher == uuid.Nil {
		return fmt.Errorf("no sender")
	}

	return nil
}

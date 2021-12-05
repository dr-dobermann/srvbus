package es

import (
	"time"

	"github.com/dr-dobermann/srvbus/internal/ds"
)

// Event represent the single event in the system which
// occurs somewhere and At given time, has name and other details in data.
type Event struct {
	ds.DataItem
	At time.Time
}

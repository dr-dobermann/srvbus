package es

import (
	"time"

	"github.com/google/uuid"
)

type EventEnvelope struct {
	event *Event

	publisher uuid.UUID
	regAt     time.Time
}

type Topic struct {
	name   string
	events []EventEnvelope
}

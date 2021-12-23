package errs

import (
	"errors"
)

var (
	ErrGrpcNoHost    = errors.New("host server isn't set")
	ErrNotRunned     = errors.New("server isn't runned")
	ErrAlreadyRunned = errors.New("already runned")

	ErrEmptyQueueName = errors.New("queue name is empty")
	ErrNoSender       = errors.New("sender is not set")
	ErrEmptyMessage   = errors.New("messages is empty")

	ErrNoService = errors.New("couldn't register a nil-service")

	ErrNoLogger = errors.New("logger isn't present")

	ErrNotImplementedYet = errors.New("not implemented yet")
)

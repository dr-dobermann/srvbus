package errs

import "fmt"

var (
	ErrGrpcNoHost    = fmt.Errorf("host server isn't set")
	ErrNotRunned     = fmt.Errorf("server isn't runned")
	ErrAlreadyRunned = fmt.Errorf("already runned")

	ErrEmptyQueueName = fmt.Errorf("queue name is empty")
	ErrNoSender       = fmt.Errorf("sender is not set")
	ErrEmptyMessage   = fmt.Errorf("messages is empty")

	ErrNoService = fmt.Errorf("couldn't register a nil-service")

	ErrNoLogger = fmt.Errorf("logger isn't present")

	ErrNotImplementedYet = fmt.Errorf("not implemented yet")
)

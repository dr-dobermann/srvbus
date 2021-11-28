package s2

import (
	"context"
	"fmt"
	"io"
)

// =============================================================================
//    Output Service

// newOutputService returns an output ServiceFunc which
// puts all values form vl into io.Writer w.
func newOutputService(
	_ context.Context,
	w io.Writer,
	vl ...interface{}) (ServiceRunner, error) {
	if w == nil {
		return nil, fmt.Errorf("writer isn't presented for OutputSvc")
	}

	outputSvc := func(_ context.Context) error {
		fmt.Fprint(w, vl...)

		return nil
	}

	return ServiceFunc(outputSvc), nil
}

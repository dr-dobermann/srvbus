package s2

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/matryer/is"
	"go.uber.org/zap"
)

func TestOutputSvc(t *testing.T) {
	is := is.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create service
	out := bytes.NewBuffer([]byte{})
	testStr := []string{"Hello ", "Dober!"}

	svc, err := newOutputService(ctx, out, testStr[0], testStr[1])
	is.NoErr(err)
	is.True(svc != nil)

	// testing invalid params
	t.Run("invalid_params", func(t *testing.T) {
		_, err := newOutputService(ctx, nil, "this is a test")
		is.True(err != nil)
	})

	// run service and check results
	err = svc.Run(ctx)
	is.NoErr(err)
	is.Equal(out.String(), strings.Join(testStr, ""))
}

func TestSvcServer(t *testing.T) {
	is := is.New(t)

	log, err := zap.NewDevelopment()
	is.NoErr(err)

	srvID := struct {
		id   uuid.UUID
		name string
	}{uuid.New(), "ServiceServer"}

	// server creation and running
	sSrv, err := New(srvID.id, srvID.name, log.Sugar())
	is.NoErr(err)
	is.True(sSrv != nil)

	is.Equal(srvID.id.String(), sSrv.ID.String())
	is.Equal(srvID.name, sSrv.Name)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = sSrv.Run(ctx)
	is.NoErr(err)

	// register service
	output := bytes.NewBuffer([]byte{})
	testStr := []string{"Hello ", "Dober!"}
	svc, err := newOutputService(ctx, output, testStr[0], testStr[1])
	is.NoErr(err)
	is.True(svc != nil)

	id, err := sSrv.AddService("OutputService", svc)
	is.NoErr(err)
	is.True(id != uuid.Nil)

	err = sSrv.ExecService(id)
	is.NoErr(err)

	for !sSrv.IsSvcFinished(id) {
	}

	is.Equal(output.String(), strings.Join(testStr, ""))

	fmt.Println(output.String())
}

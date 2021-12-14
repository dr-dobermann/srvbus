package s2

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/dr-dobermann/srvbus/es"
	"github.com/google/uuid"
	"github.com/matryer/is"
	"go.uber.org/zap"
)

func TestSvcServer(t *testing.T) {
	is := is.New(t)

	log, err := zap.NewDevelopment()
	is.NoErr(err)

	srvID := struct {
		id   uuid.UUID
		name string
	}{uuid.New(), "ServiceServer"}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eSrv, err := es.New(uuid.New(), "EVT_SRV", log.Sugar())
	is.NoErr(err)
	is.True(eSrv != nil)
	is.NoErr(eSrv.Run(ctx, false))

	// server creation and running
	sSrv, err := New(srvID.id, srvID.name, log.Sugar(), eSrv)
	is.NoErr(err)
	is.True(sSrv != nil)

	is.Equal(srvID.id.String(), sSrv.ID.String())
	is.Equal(srvID.name, sSrv.Name)

	err = sSrv.Run(ctx)
	is.NoErr(err)

	// register service
	output := bytes.NewBuffer([]byte{})
	testStr := []string{"Hello ", "Dober!"}
	svc, err := NewOutputService(ctx, output, testStr[0], testStr[1])
	is.NoErr(err)
	is.True(svc != nil)

	id, err := sSrv.AddService("OutputService", svc, nil, false)
	is.NoErr(err)
	is.True(id != uuid.Nil)

	fmt.Println(sSrv.Stat().String())

	err = sSrv.ExecService(id)
	is.NoErr(err)

	t.Run("invalid_runners", func(t *testing.T) {
		// starting non-existing service
		err := sSrv.ExecService(uuid.New())
		is.True(err != nil)

		// starting executed service
		err = sSrv.ExecService(id)
		is.True(err != nil)
	})

	// wait for invalid service
	_, err = sSrv.WaitForService(ctx, uuid.New())
	is.True(err != nil)

	// wait for single service
	singleSvc, err := sSrv.WaitForService(ctx, id)
	is.NoErr(err)

	res := <-singleSvc
	fmt.Printf("  => svc #%v is finished. Success: %t", id, res)

	// wait for all services
	allSvc, err := sSrv.WaitForService(ctx, uuid.Nil)
	is.NoErr(err)
	res = <-allSvc
	fmt.Printf("  => ALL svcs are finished. Success: %t", res)

	is.Equal(output.String(), strings.Join(testStr, ""))

	fmt.Println(output.String())

	fmt.Println(sSrv.Stat().String())

}

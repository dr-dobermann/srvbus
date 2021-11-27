package s2

import (
	"testing"

	"github.com/google/uuid"
	"github.com/matryer/is"
	"go.uber.org/zap"
)

func TestSvcServer(t *testing.T) {
	is := is.New(t)

	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()

	log, err := zap.NewDevelopment()
	is.NoErr(err)

	srvID := struct {
		id   uuid.UUID
		name string
	}{uuid.New(), "ServiceServer"}

	sSrv, err := New(srvID.id, srvID.name, log.Sugar())
	is.NoErr(err)
	is.True(sSrv != nil)

	is.Equal(srvID.id.String(), sSrv.ID.String())
	is.Equal(srvID.name, sSrv.Name)
}

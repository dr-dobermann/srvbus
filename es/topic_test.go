package es

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/matryer/is"
	"go.uber.org/zap"
)

func getServer(id uuid.UUID, name string, t *testing.T) *EventServer {
	log, err := zap.NewDevelopment()
	if err != nil {
		t.Fatal(err.Error())
	}

	eSrv, err := New(id, name, log.Sugar())
	if err != nil {
		t.Fatal(err.Error())
	}

	return eSrv
}

func TestTopicsTree(t *testing.T) {
	is := is.New(t)
	eSrv := getServer(uuid.Nil, "", t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	is.NoErr(eSrv.Run(ctx, false))

	err := eSrv.AddTopic("main", "")
	is.NoErr(err)

	is.True(eSrv.HasTopic("/main"))

	is.NoErr(eSrv.AddTopic("subtopic", "/main"))

	is.NoErr(eSrv.AddTopic("subsubtopic", "/main/subtopic/"))

	is.True(eSrv.HasTopic("/main/subtopic/subsubtopic/"))

	t.Run("check_invalid_conditions", func(t *testing.T) {
		// add duplicate root topic
		err := eSrv.AddTopic("main", "/")
		is.True(err != nil)

		// add duplicate topic
		is.True(eSrv.AddTopic("subsubtopic", "/main/subtopic/") != nil)

		// check EventServiceError Error()
		if err != nil {
			fmt.Println(err.Error())
		}

		// add to invalid tree
		is.True(eSrv.AddTopic("subtopic", "/mani") != nil)

		// check adding with invalid subtopic
		is.True(eSrv.AddTopic("sstopic", "/main/ssubtopic") != nil)

		// check invalide root topic
		is.True(!eSrv.HasTopic("mani"))

		// check invalid subtree
		is.True(!eSrv.HasTopic("/main/ssss"))

		// add empty topic
		is.True(eSrv.AddTopic("", "/") != nil)

		// check empty topic
		is.True(!eSrv.HasTopic("/"))
	})

	// check topics branch adding
	is.NoErr(eSrv.AddTopicQueue("st/sst/ssst", "/main/"))
	is.True(eSrv.HasTopic("/main/st/sst/ssst/"))
	// duplicate topic
	is.True(eSrv.AddTopicQueue("ssst/sssst", "/main/st/sst") != nil)
	// no branch
	is.True(eSrv.AddTopicQueue("ssst/sssst", "/main/sst") != nil)
}

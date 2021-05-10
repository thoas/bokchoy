package bokchoy_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/thoas/bokchoy"
	"github.com/thoas/bokchoy/logging"
	"github.com/thoas/bokchoy/middleware"
)

type suiteServer struct {
}

func (s suiteServer) Start(context.Context) error {
	return nil
}

func (s suiteServer) Stop(context.Context) {
}

type suite struct {
	bokchoy *bokchoy.Bokchoy
}

type FuncTest func(t *testing.T, s *suite)

// nolint
func run(t *testing.T, f FuncTest) {
	logger, err := logging.NewDevelopmentLogger()
	if err != nil {
		log.Fatal(err)
	}

	defer logger.Sync()

	ctx := context.Background()

	addr := fmt.Sprintf("%s:%s", os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT"))

	bok, err := bokchoy.New(ctx, bokchoy.Config{
		Broker: bokchoy.BrokerConfig{
			Type: "redis",
			Redis: bokchoy.RedisConfig{
				Type: "client",
				Client: bokchoy.RedisClientConfig{
					Addr: addr,
				},
			},
		},
		Queues: []bokchoy.QueueConfig{
			{
				Name: "tests.task.message",
			},
		},
	},
		bokchoy.WithQueues([]string{"tasks.message"}),
		bokchoy.WithServers([]bokchoy.Server{suiteServer{}}),
		bokchoy.WithLogger(logger.With(logging.String("logger", "bokchoy"))))
	if err != nil {
		panic(err)
	}

	bok.Use(middleware.RequestID)
	bok.Use(middleware.Recoverer)

	err = bok.Empty(ctx)
	if err != nil {
		panic(err)
	}

	suite := &suite{bok}

	f(t, suite)
}

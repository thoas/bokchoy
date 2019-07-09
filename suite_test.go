package bokchoy_test

import (
	"context"
	"log"
	"testing"

	"github.com/thoas/bokchoy"
	"github.com/thoas/bokchoy/logging"
	"github.com/thoas/bokchoy/middleware"
)

type suite struct {
	bokchoy *bokchoy.Bokchoy
}

type FuncTest func(t *testing.T, s *suite)

func run(t *testing.T, f FuncTest) {
	logger, err := logging.NewDevelopmentLogger()
	if err != nil {
		log.Fatal(err)
	}

	defer logger.Sync()

	ctx := context.Background()

	bok, err := bokchoy.New(ctx, bokchoy.Config{
		Broker: bokchoy.BrokerConfig{
			Type: "redis",
			Redis: bokchoy.RedisConfig{
				Type: "client",
				Client: bokchoy.RedisClientConfig{
					Addr: "localhost:6379",
				},
			},
		},
		Queues: []bokchoy.QueueConfig{
			{
				Name: "tests.task.message",
			},
		},
	}, bokchoy.WithLogger(logger.With(logging.String("logger", "bokchoy"))))
	bok.Use(middleware.RequestID)
	bok.Use(middleware.Recoverer)

	err = bok.Empty(ctx)
	if err != nil {
		panic(err)
	}

	suite := &suite{bok}

	f(t, suite)
}

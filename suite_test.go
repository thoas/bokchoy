package bokchoy

import (
	"context"
	"log"
	"testing"

	"github.com/thoas/bokchoy/logging"
)

type suite struct {
	bokchoy *Bokchoy
}

type FuncTest func(t *testing.T, s *suite)

func run(t *testing.T, f FuncTest) {
	logger, err := logging.NewDevelopmentLogger()
	if err != nil {
		log.Fatal(err)
	}

	defer logger.Sync()

	ctx := context.Background()

	bok, err := New(ctx, Config{
		Broker: BrokerConfig{
			Type: "redis",
			Redis: RedisConfig{
				Type: "client",
				Client: RedisClientConfig{
					Addr: "localhost:6379",
				},
			},
		},
		Queues: []QueueConfig{
			{
				Name: "tests.task.message",
			},
		},
	}, WithLogger(logger.With(logging.String("logger", "bokchoy"))))

	err = bok.Empty(ctx)
	if err != nil {
		panic(err)
	}

	suite := &suite{bok}

	f(t, suite)
}

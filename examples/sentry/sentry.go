package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/getsentry/sentry-go"
	"github.com/thoas/bokchoy"
	bokchoysentry "github.com/thoas/bokchoy/contrib/sentry"
	"github.com/thoas/bokchoy/middleware"
)

func main() {
	var (
		err error
		ctx = context.Background()
		run string
	)

	flag.StringVar(&run, "run", "", "service to run")
	flag.Parse()

	sentry.Init(sentry.ClientOptions{
		Dsn: os.Getenv("SENTRY_DSN"),
	})

	engine, err := bokchoy.New(ctx, bokchoy.Config{
		Broker: bokchoy.BrokerConfig{
			Type: "redis",
			Redis: bokchoy.RedisConfig{
				Type: "client",
				Client: bokchoy.RedisClientConfig{
					Addr: "localhost:6379",
				},
			},
		},
	}, bokchoy.WithTracer(&bokchoysentry.SentryTracer{}))
	if err != nil {
		log.Fatal(err)
	}

	engine.Use(middleware.Recoverer)

	switch run {
	case "producer":
		task, err := engine.Queue("tasks.message").Publish(ctx, map[string]string{
			"data": "hello world",
		})
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(task, "has been published")
	case "worker":
		engine.Queue("tasks.message").HandleFunc(func(r *bokchoy.Request) error {
			return fmt.Errorf("Unexpected error")
		})

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)

		go func() {
			for range c {
				log.Print("Received signal, gracefully stopping")
				engine.Stop(ctx)
			}
		}()

		engine.Run(ctx)
	}

}

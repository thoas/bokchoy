package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/thoas/bokchoy"
	"github.com/thoas/bokchoy/logging"
	"github.com/thoas/bokchoy/middleware"
)

func main() {
	var (
		logger      logging.Logger
		ctx         = context.Background()
		loggerLevel = os.Getenv("LOGGER_LEVEL")
	)

	if loggerLevel == "development" {
		logger, err := logging.NewDevelopmentLogger()
		if err != nil {
			log.Fatal(err)
		}

		defer logger.Sync()
	}

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
	}, bokchoy.WithLogger(logger))
	if err != nil {
		log.Fatal(err)
	}

	engine.Use(middleware.Recoverer)
	engine.Use(middleware.DefaultLogger)
	engine.Use(middleware.RequestID)

	engine.Queue("tasks.message").HandleFunc(func(r *bokchoy.Request) error {
		fmt.Println("Receive request:", r)
		fmt.Println("Request context:", r.Context())
		fmt.Println("Payload:", r.Task.Payload)

		r.Task.Result = "You can store your result here"

		return nil
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

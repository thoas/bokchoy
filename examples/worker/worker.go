package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/thoas/bokchoy"
	"github.com/thoas/bokchoy/middleware"
)

func main() {
	ctx := context.Background()

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
	})
	if err != nil {
		log.Fatal(err)
	}

	engine.Use(middleware.Recoverer)
	engine.Use(middleware.RequestID)
	engine.Use(middleware.DefaultLogger)

	engine.Queue("tasks.message").SubscribeFunc(func(r *bokchoy.Request) error {
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

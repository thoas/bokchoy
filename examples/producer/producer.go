package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/thoas/bokchoy"
)

func main() {
	ctx := context.Background()

	// define the main engine which will manage queues
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

	payload := map[string]string{
		"data": "hello world",
	}

	task, err := engine.Queue("tasks.message").Publish(ctx, payload,
		bokchoy.WithTimeout(1*time.Second), bokchoy.WithCountdown(-1))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(task, "has been published")
}

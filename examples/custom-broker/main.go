package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v7"

	"github.com/thoas/bokchoy"
	"github.com/thoas/bokchoy/logging"
)

func main() {
	ctx := context.Background()

	logger := logging.NewNopLogger()

	clt := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// define the main engine which will manage queues
	engine, err := bokchoy.New(ctx, bokchoy.Config{}, bokchoy.WithBroker(&bokchoy.RedisBroker{
		Client:     clt,
		ClientType: "client",
		Logger:     logger,
	}))
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

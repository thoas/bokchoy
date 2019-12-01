package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/mitchellh/mapstructure"

	"github.com/thoas/bokchoy"
	"github.com/thoas/bokchoy/logging"
	"github.com/thoas/bokchoy/middleware"
)

type message struct {
	Data string `json:"data"`
}

// nolint: gocyclo, vet
func main() {
	var (
		run            string
		taskID         string
		retryIntervals string
		concurrency    int
	)

	flag.StringVar(&taskID, "task-id", "", "task identifier")
	flag.StringVar(&run, "run", "publish", "service to run")
	flag.StringVar(&retryIntervals, "retry-intervals", "2,4,6", "retry intervals in seconds")
	flag.IntVar(&concurrency, "concurrency", 1, "concurrency to run consumer")
	flag.Parse()

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
	}, bokchoy.WithLogger(logger.With(logging.String("logger", "bokchoy"))))
	bok.Use(middleware.RequestID)

	queue := bok.Queue("tasks.message")
	queueFail := bok.Queue("tasks.message.failed")

	retryIntervalsList := strings.Split(retryIntervals, ",")
	intervals := make([]time.Duration, len(retryIntervalsList))
	for i := range retryIntervalsList {
		value, _ := strconv.ParseInt(retryIntervalsList[i], 10, 64)

		intervals[i] = time.Duration(value) * time.Second
	}

	switch run {
	case "list":
		tasks, err := queue.List(ctx)

		if err != nil {
			log.Fatal(err)
		}

		for i := range tasks {
			log.Printf("%s retrieved", tasks[i])
		}
	case "get":
		task, err := queue.Get(ctx, taskID)

		if err != nil {
			log.Fatal(err)
		}

		log.Printf("%s retrieved", task)
	case "cancel":
		task, err := queue.Cancel(ctx, taskID)

		if err != nil {
			log.Fatal(err)
		}

		log.Printf("%s canceled", task)
	case "publish:failed:intervals":
		if err != nil {
			log.Fatal(err)
		}

		task, err := queueFail.Publish(ctx, message{Data: "hello"},
			bokchoy.WithMaxRetries(3),
			bokchoy.WithRetryIntervals(intervals))

		if err != nil {
			log.Fatal(err)
		}

		log.Printf("%s published with retry intervals", task)
	case "publish:delay":
		if err != nil {
			log.Fatal(err)
		}

		task, err := queue.Publish(ctx, message{Data: "hello"},
			bokchoy.WithCountdown(5*time.Second))
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("%s delayed published", task)
	case "publish":
		if err != nil {
			log.Fatal(err)
		}

		for i := 0; i < concurrency; i++ {
			task, err := queue.Publish(ctx, message{Data: "hello"})
			if err != nil {
				log.Fatal(err)
			}

			log.Printf("%s published", task)
		}

	case "publish:timeout":
		if err != nil {
			log.Fatal(err)
		}

		task, err := queue.Publish(ctx, message{Data: "hello"},
			bokchoy.WithTimeout(5*time.Second))
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("%s published", task)
	case "consume":
		queueFail.HandleFunc(func(r *bokchoy.Request) error {
			return fmt.Errorf("It should fail badly")
		}, bokchoy.WithConcurrency(concurrency))

		queue.OnStartFunc(func(r *bokchoy.Request) error {
			*r = *r.WithContext(context.WithValue(r.Context(), "foo", "bar"))

			return nil
		})

		queue.OnCompleteFunc(func(r *bokchoy.Request) error {
			spew.Dump(r.Context())

			return nil
		})

		queue.HandleFunc(func(r *bokchoy.Request) error {
			var (
				msg  message
				task = r.Task
			)
			err := mapstructure.Decode(task.Payload, &msg)
			if err != nil {
				return err
			}

			log.Printf("%s received, message decoded: %+v", task, msg)

			return nil
		}, bokchoy.WithConcurrency(concurrency))

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)

		go func() {
			for range c {
				log.Print("Received signal, gracefully stopping")
				bok.Stop(ctx)
			}
		}()

		bok.Run(ctx)
	}
}

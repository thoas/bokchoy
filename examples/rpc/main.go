package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/thoas/bokchoy"
	"github.com/thoas/bokchoy/contrib/rpc"
	"github.com/thoas/bokchoy/middleware"
	"google.golang.org/grpc/codes"
)

const (
	queueName = "tasks.message"
)

func main() {
	var (
		err     error
		ctx     = context.Background()
		run     string
		rpcPort int
	)

	flag.StringVar(&run, "run", "", "service to run")
	flag.IntVar(&rpcPort, "rpc-port", 9090, "port for rpc server")
	flag.Parse()

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

	engine.Use(middleware.DefaultLogger)

	switch run {
	case "client":
		clt := rpc.NewClient(fmt.Sprintf(":%d", rpcPort), rpc.ClientOptions{
			MaxRetries:      3,
			PerRetryTimeout: 1 * time.Second,
			RetryCodes:      []codes.Code{codes.Unavailable},
		})

		task, err := clt.PublishTask(ctx, queueName, []byte(`{"data": "hello world"}`))
		if err != nil {
			log.Fatalf("could not retrieve result: %v", err)
		}

		log.Printf("Task published: %+v", task)
	case "producer":
		task, err := engine.Queue(queueName).Publish(ctx, map[string]string{
			"data": "hello world",
		})
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(task, "has been published")
	case "worker":
		engine.Queue(queueName).HandleFunc(func(r *bokchoy.Request) error {
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

		engine.Run(ctx, bokchoy.WithServices([]bokchoy.Service{
			rpc.NewServer(engine, rpcPort),
		}))
	}
}

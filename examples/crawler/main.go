package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/thoas/bokchoy"
	"github.com/thoas/bokchoy/examples/crawler/handler"
	"github.com/thoas/bokchoy/examples/crawler/parser"
	"github.com/thoas/bokchoy/logging"
	"github.com/thoas/bokchoy/middleware"
)

func main() {
	var (
		// which service needs to be run
		run string

		// url to crawl
		url string

		// until depth
		depth int

		// timeout
		timeout int

		// concurrency
		concurrency int

		// redis address to customize
		redisAddr   string
		err         error
		ctx         = context.Background()
		logger      logging.Logger
		loggerLevel = os.Getenv("LOGGER_LEVEL")
	)

	if loggerLevel == "development" {
		logger, err = logging.NewDevelopmentLogger()
		if err != nil {
			log.Fatal(err)
		}

		defer logger.Sync()
	}

	flag.IntVar(&depth, "depth", 1, "depth to crawl")
	flag.IntVar(&timeout, "timeout", 5, "timeout in seconds")
	flag.IntVar(&concurrency, "concurrency", 1, "number of workers")
	flag.StringVar(&url, "url", "", "url to crawl")
	flag.StringVar(&run, "run", "", "service to run")
	flag.StringVar(&redisAddr, "redis-addr", "localhost:6379", "redis address")
	flag.Parse()

	bok, err := bokchoy.New(ctx, bokchoy.Config{
		Broker: bokchoy.BrokerConfig{
			Type: "redis",
			Redis: bokchoy.RedisConfig{
				Type: "client",
				Client: bokchoy.RedisClientConfig{
					Addr: redisAddr,
				},
			},
		},
	}, bokchoy.WithMaxRetries(2), bokchoy.WithRetryIntervals([]time.Duration{
		5 * time.Second,
		10 * time.Second,
	}), bokchoy.WithLogger(logger))
	bok.Use(middleware.Recoverer)
	bok.Use(middleware.DefaultLogger)

	queue := bok.Queue("tasks.crawl")

	if err != nil {
		log.Fatal(err)
	}

	h := handler.NewCrawlHandler(queue, &parser.DocumentParser{}, time.Duration(timeout))

	switch run {
	case "producer":
		task, err := h.Crawl(ctx, url, url, depth)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("%s published", task)
	case "worker":
		queue.Handle(h, bokchoy.WithConcurrency(concurrency))

		// initialize a signal to close Bokchoy
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)

		// iterate over the channel to stop
		go func() {
			for range c {
				log.Print("Received signal, gracefully stopping")
				bok.Stop(ctx)
			}
		}()

		// blocking operation, everything is done for you
		bok.Run(ctx)
	}
}

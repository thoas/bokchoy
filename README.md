# bokchoy

[![Build Status](https://travis-ci.org/thoas/bokchoy.svg?branch=master)](https://travis-ci.org/thoas/bokchoy)
[![GoDoc](https://godoc.org/github.com/thoas/bokchoy?status.svg)](https://godoc.org/github.com/thoas/bokchoy)
[![Go report](https://goreportcard.com/badge/github.com/thoas/bokchoy)](https://goreportcard.com/report/github.com/thoas/bokchoy)

## Introduction

Bokchoy is a simple Go library for queueing tasks and processing them in the background with workers.
It should be integrated in your web stack easily and it's designed to have a low barrier entry for newcomers.

It currently only supports [Redis](https://github.com/thoas/bokchoy/blob/master/broker_redis.go)
(client, sentinel and cluster) with some Lua magic, but internally it relies on a generic
broker implementation to extends it.

![screen](https://d2aztkdj0ezvrk.cloudfront.net/items/392d0G1F3D2y1r2i0r1M/screen.gif)

## Motivation

It's relatively easy to make a producer/receiver system in Go since the language contains builtins
features to build it from scratch but we keep adding the same system everywhere instead of thinking reusable.

Bokchoy is a plug and play component, it does its job and it does it well for you that you can focus
on your business logic.

## Features

* **Lightweight**
* **A Simple API close to net/http** - if you already use `net/http` then you can learn it pretty quickly
* **Designed with a modular/composable APIs** - middlewares, queue middlewares
* **Context control** - built on `context` package, providing value chaining, cancelations and timeouts
* **Highly configurable** - tons of options to swap internal parts (broker, logger, timeouts, etc), if you cannot customize something then an option is missing

## Getting started

First, run a Redis server, of course:

```console
redis-server
```

Define your producer which will send tasks:

```go
package main

import (
	"context"
	"fmt"
	"log"

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

	task, err := engine.Queue("tasks.message").Publish(ctx, payload)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(task, "has been published")
}
```

See [producer](examples/producer) directory for more information and to run it.

Now we have a producer which can send tasks to our engine, we need a worker to process
them in the background:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/thoas/bokchoy"
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

	engine.Queue("tasks.message").HandleFunc(func(r *bokchoy.Request) error {
		fmt.Println("Receive request", r)
		fmt.Println("Payload:", r.Task.Payload)

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
```

A worker is defined by handlers, to define a `Handler` you have to follow this interface:

```go
type Handler interface {
	Handle(*Request) error
}
```

You can create your own struct which implements this interface or use the `HandlerFunc` to
generate a `Handler` from your function.

See [worker](examples/worker) directory for more information and to run it.

## Installation

Using [Go Modules](https://github.com/golang/go/wiki/Modules)

```console
go get github.com/thoas/bokchoy
```

## Advanced topics

### Delayed task

When publishing a task, it will be immediately processed by the worker if it's not already occupied,
you may want to delay the task on some occasions by using `bokchoy.WithCountdown` option:

```go
payload := map[string]string{
    "data": "hello world",
}

queue.Publish(ctx, payload, bokchoy.WithCountdown(5*time.Second))
```

This task will be executed in 5 seconds.

### Custom serializer

By default the task serializer is `JSON`, you can customize it when initializing
the Bokchoy engine, it must respect the
[Serializer](https://github.com/thoas/bokchoy/blob/master/serializer.go) interface.

```go
bokchoy.New(ctx, bokchoy.Config{
    Broker: bokchoy.BrokerConfig{
        Type: "redis",
        Redis: bokchoy.RedisConfig{
            Type: "client",
            Client: bokchoy.RedisClientConfig{
                Addr: "localhost:6379",
            },
        },
    },
}, bokchoy.WithSerializer(MySerializer{}))
```

You will be capable to define a [msgpack](https://msgpack.org/), [yaml](https://yaml.org/) serializers if you want.

### Custom logger

By default the internal logger is disabled, you can provide a more verbose logger with options:

```go
import (
	"context"
	"fmt"
	"log"

	"github.com/thoas/bokchoy/logging"
)

func main() {
	logger, err := logging.NewDevelopmentLogger()
	if err != nil {
		log.Fatal(err)
	}

	defer logger.Sync()

    bokchoy.New(ctx, bokchoy.Config{
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
}
```

The builtin logger is based on [zap](https://github.com/uber-go/zap) but you can provide your
own implementation easily if you have a central component.

If you don't need that much information, you can enable the [Logger middleware](#core-middlewares).

### Worker Concurrency

By default the worker concurrency is set to `1`, you can override it based on your server
capability, Bokchoy will spawn multiple goroutines to handle your tasks.

```go
engine.Queue("tasks.message").HandleFunc(func(r *bokchoy.Request) error {
    fmt.Println("Receive request", r)
    fmt.Println("Payload:", r.Task.Payload)

    return nil
}, bokchoy.WithConcurrency(5))
```

### Retries

If your task handler is returning an error, the task will be marked as `failed` and retried `3 times`,
based on intervals: `60 seconds`, `120 seconds`, `180 seconds`.

You can customize this globally on the engine or when publishing a new task by using `bokchoy.WithMaxRetries`
and `bokchoy.WithRetryIntervals` options.

```go
bokchoy.WithMaxRetries(1)
bokchoy.WithRetryIntervals([]time.Duration{
	180 * time.Second,
})
```

### Timeout

By default a task will be forced to timeout and marked as `canceled` if its running time exceed `180 seconds`.

You can customize this globally or when publishing a new task by using `bokchoy.WithTimeout` option:

```go
bokchoy.WithTimeout(5*time.Second)
```

The worker will regain control and process the next task but be careful, each task is running
in a goroutine so you have to cancel your task at some point or it will be leaking.

### Catch events

You can catch events by registering handlers on your queue when your tasks are
starting, succeeding, completing or failing.

```go
queue := engine.Queue("tasks.message")
queue.OnStartFunc(func(r *bokchoy.Request) error {
    // we update the context by adding a value
    *r = *r.WithContext(context.WithValue(r.Context(), "foo", "bar"))

    return nil
})

queue.OnCompleteFunc(func(r *bokchoy.Request) error {
    fmt.Println(r.Context().Value("foo"))

    return nil
})

queue.OnSuccessFunc(func(r *bokchoy.Request) error {
    fmt.Println(r.Context().Value("foo"))

    return nil
})

queue.OnFailureFunc(func(r *bokchoy.Request) error {
    fmt.Println(r.Context().Value("foo"))

    return nil
})
```

### Store results

By default, if you don't mutate the task in the handler its result will be always `nil`.

You can store a result in your task to keep it for later, for example: you might need statistics from a twitter profile
to save them later.

```go
queue.HandleFunc(func(r *bokchoy.Request) error {
	r.Task.Result = map[string]string{"result": "wow!"}

	return nil
})
```

You can store anything as long as your serializer can serializes it.

Keep in mind the default task TTL is `180 seconds`, you can override it with `bokchoy.WithTTL` option.

### Helpers

Let's define our previous queue:

```go
queue := engine.Queue("tasks.message")
```

#### Empty the queue

```go
queue.Empty()
```

It will remove all waiting tasks from your queue.

#### Cancel a waiting task

We produce a task without running the worker:

```go
payload := map[string]string{
    "data": "hello world",
}

task, err := queue.Publish(ctx, payload)
if err != nil {
    log.Fatal(err)
}
```

Then we can cancel it by using its ID:

```go
queue.Cancel(ctx, task.ID)
```

#### Retrieve a published task from the queue

```go
queue.Get(ctx, task.ID)
```

#### Retrieve statistics from a queue

```go
stats, err := queue.Count(ctx)
if err != nil {
    log.Fatal(err)
}

fmt.Println("Number of waiting tasks:", stats.Direct)
fmt.Println("Number of delayed tasks:", stats.Delayed)
fmt.Println("Number of total tasks:", stats.Total)
```

## Middleware handlers

Bokchoy comes equipped with an optional middleware package, providing a suite of standard middlewares.
Middlewares have the same API as handlers. It's easy to implement them and think of them like `net/http` middlewares,
they share the same purpose to follow the lifecycle of a Bokchoy request.

### Core middlewares

-----------------------------------------------------------------------------------------------------------
| bokchoy/middleware    | description                                                                     |
|:----------------------|:---------------------------------------------------------------------------------
| Logger                | Logs the start and end of each request with the elapsed processing time         |
| Recoverer             | Gracefully absorb panics and prints the stack trace                             |
| RequestID             | Injects a request ID into the context of each request                           |
| Timeout               | Signals to the request context when the timeout deadline is reached             |
-----------------------------------------------------------------------------------------------------------

See [middleware](middleware) directory for more information.


## FAQs

### Are Task IDs unique?

Yes! There are based on [ulid](https://github.com/oklog/ulid).

### Is exactly-once execution of tasks guaranteed?

It's guaranteed by the underlying broker,
it uses [BRPOP](https://redis.io/commands/brpop)/[BLPOP](https://redis.io/commands/blpop) from Redis.

If multiple clients are blocked for the same key, the first client to be served
is the one that was waiting for more time (the first that blocked for the key).

## Contributing

* Ping me on twitter:
  * [@thoas](https://twitter.com/thoas)
* Fork the [project](https://github.com/thoas/bokchoy)
* Fix [bugs](https://github.com/thoas/bokchoy/issues)

**Don't hesitate ;)**

## Project history

Bokchoy is highly influenced by the great [rq](https://github.com/rq/rq) and [celery](http://www.celeryproject.org/).

Both are great projects well maintained but only used in a Python ecosystem.

Some parts (middlewares mostly) of Bokchoy are heavily inspired or taken from [go-chi](https://github.com/go-chi/chi).

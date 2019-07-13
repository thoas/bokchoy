# A Tour of Bokchoy, a simple job queues for Go backed by Redis

Bokchoy is a simple Go library for queueing tasks and processing them in the background with workers.
It can be used in multiple cases: crawling third party APIs, slow processes, compute, analysis, etc. 

It should be integrated in any web stack easily and it's designed to have a low barrier entry for newcomers.

To demonstrate each feature, we will create a minimalist and dumb web crawler
inspired by the one found in [A Tour of Go](https://tour.golang.org/concurrency/10)
with Bokchoy, links will be distributed around multiple servers.

TL;DR: the complete application can be found [here](../examples/crawler)

## Overview

We will start small and go deeper and deeper, to crawl a website we need:

* A base URL to start crawling
* A `depth` parameter to stop the crawler when it's too deep
* An urls collector to extract urls from a webpage and propagate them to subtasks
* A common storage to list all urls found and theirs statuses

## Installation

First, run a Redis server:

```console
$ redis-server
```

Ensure Go is installed:

```console
$ go version
go version go1.12.6 darwin/amd64
```

Export `GO111MODULE=on` globally to let Go engine install dependencies:

```console
$ export GO111MODULE=on
```

## Setup

We will export our code in a single file named `main.go` to keep it readable for this tutorial
and iterate over it, step by step.

Define an initial `Crawl` structure:

```go
// main.go
package main

import (
	"fmt"
)

// Crawl defines a crawl.
type Crawl struct {
	URL   string `json:"url"`
	Depth int    `json:"depth"`
}

// Strings returns string representation of a crawl.
func (c Crawl) String() string {
	return fmt.Sprintf(
		"<Crawl url=%s depth=%d>",
		c.URL, c.Depth)
}

func main() {
}
```

In order to publish an initial URL to crawl, we use [flag](https://golang.org/pkg/flag/) package:

```go
func main() {
	var (
		// which service needs to be run
		run string

		// url to crawl
		url string

		// until depth
		depth int

		// redis address to customize
		redisAddr string
	)

	flag.IntVar(&depth, "depth", 1, "depth to crawl")
	flag.StringVar(&url, "url", "", "url to crawl")
	flag.StringVar(&run, "run", "", "service to run")
	flag.StringVar(&redisAddr, "redis-addr", "localhost:6379", "redis address")
	flag.Parse()
}
```

Our CLI API to produce a new task:

```console
$ go run main.go -run producer -url {url} -depth {depth}
```

Bokchoy is a complete engine which exposes queues to publish:

```go
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
})
```

Multiple syntax can be used to publish a new task to a queue:

```go
err := bok.Queue("tasks.crawl").Publish(ctx, &Crawl{URL: url, Depth: depth})
if err != nil {
	log.Fatal(err)
}
```

or

```go
err := bok.Publish(ctx, "tasks.crawl", &Crawl{URL: url, Depth: depth})
if err != nil {
	log.Fatal(err)
}
```

We use the `run` variable to implement the producer service:

```go
// ...

queue := bok.Queue("tasks.crawl")

switch run {
case "producer":
	task, err := queue.Publish(ctx, &Crawl{
		URL:   url,
		Depth: depth,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("%s published", task)
}
```

We can now test the producer by running:

```console
$ go run main.go -run producer -url https://www.ulule.com
2019/07/10 17:22:14 <Task name=tasks.crawl id=01DFE7MPKHA8YVF26PTC1RFSCV, status=waiting, published_at=2019-07-10 15:22:14.513543 +0000 UTC> published
```

The task is in `waiting` state, we need a worker to process it.

It remains in the broker until it's completely processed (`failed` or `succeeded`)
then it's be kept in the broker with a default TTL as `180 seconds`.
This duration can be customized globally on the engine
or per tasks on the publish statement.

The `bokchoy.WithTTL` option customizes this duration:

```go
queue.Publish(ctx, &Crawl{
	URL:   url,
	Depth: depth,
}, bokchoy.WithTTL(5*time.Minute))
```

As a result, the following task is kept `5 minutes` after being processed.

The first implementation of the worker is basic and only output the task:

```go
// ...

switch run {
// ...

case "worker":
	// initialize a signal to close Bokchoy
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	// iterate over the channel to stop
	go func() {
		for range c {
			log.Print("Received signal, gracefully stopping")

			// gracefully shutdown consumers
			bok.Stop(ctx)
		}
	}()

	queue.HandleFunc(func(r *bokchoy.Request) error {
		// double marshalling to avoid casting
		// we can also use https://github.com/mitchellh/mapstructure
		res, err := json.Marshal(r.Task.Payload)
		if err != nil {
			return err
		}

		var crawl Crawl

		err = json.Unmarshal(res, &crawl)
		if err != nil {
			return err
		}

		log.Print("Received ", crawl)

		return nil
	})

	// blocking operation, everything is done for you
	bok.Run(ctx)
}
```

Launch the worker:

```console
$ go run docs/main.go -run worker
2019/07/10 17:28:47 Received <Crawl url=https://fr.ulule.com depth=1>
```

## Error handling

If the handler function doesn't return an error, the task is
marked as `succeeded`, but what's happening when an error occurred?

The handler is replaced by this one:

```go
// ...

queue.HandleFunc(func(r *bokchoy.Request) error {
	log.Print("Received ", r)

	return fmt.Errorf("An unexpected error has happened")
})
```

If the worker again is run again, three attempts to process the task are dispatched:

```console
$ go run docs/main.go -run worker
2019/07/10 17:35:27 Received <Request task: <Task name=tasks.crawl id=01DFE8CTSW4MK6453XF8SZYBZ4, status=processing, published_at=2019-07-10 15:35:25 +0000 UTC>>
2019/07/10 17:36:27 Received <Request task: <Task name=tasks.crawl id=01DFE8CTSW4MK6453XF8SZYBZ4, status=processing, published_at=2019-07-10 15:35:25 +0000 UTC>>
2019/07/10 17:37:14 Received <Request task: <Task name=tasks.crawl id=01DFE8AM02PDGADJK3FE5K6VE2, status=processing, published_at=2019-07-10 15:35:25 +0000 UTC>>
2019/07/10 17:38:27 Received <Request task: <Task name=tasks.crawl id=01DFE8CTSW4MK6453XF8SZYBZ4, status=processing, published_at=2019-07-10 15:35:25 +0000 UTC>>
```

By default, Bokchoy retries three times the task with
the following intervals: `60 seconds`, `120 seconds`, `180 seconds`.
Finally, the task is marked as `failed` in the broker.

We customize it globally by reducing intervals and the number of retries:

```go
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
}))
```

Failed tasks are handled but a panic can happen in Go and we don't want our worker to crash in this case.

Bokchoy comes equipped with a middleware package, providing a suite of standard middlewares.
Middlewares have the same API as handlers. It's easy to implement them and
can be assimiliate as net/http middlewares, they share the same purpose to
follow the lifecycle of a Bokchoy request and interact with it.

The previous handler is rewritten to panic:

```go
// ...

queue.HandleFunc(func(r *bokchoy.Request) error {
	log.Print("Received ", r)

	panic("An unexpected error has happened")
	return nil
})
```

The worker exits and fails miserably:

```console
$ go run docs/main.go -run worker
2019/07/10 17:57:52 Received <Request task: <Task name=tasks.crawl id=01DFE9NW0BTFBXQ2M3S5W2M1V4, status=processing, published_at=2019-07-10 15:57:49 +0000 UTC>>
panic: An unexpected error has happened

goroutine 42 [running]:
main.main.func2(0xc000128200, 0x0, 0x0)
        /Users/thoas/Sites/golang/src/github.com/thoas/bokchoy/docs/main.go:109 +0x98
github.com/thoas/bokchoy.HandlerFunc.Handle(0x1395618, 0xc000128200, 0x0, 0x0)
        /Users/thoas/Sites/golang/src/github.com/thoas/bokchoy/handler.go:8 +0x30
github.com/thoas/bokchoy.(*consumer).handleTask.func1(0xc00017a000, 0xc000128200, 0xc0000aaf40, 0xc0000ae360)
        /Users/thoas/Sites/golang/src/github.com/thoas/bokchoy/consumer.go:54 +0x42
created by github.com/thoas/bokchoy.(*consumer).handleTask
        /Users/thoas/Sites/golang/src/github.com/thoas/bokchoy/consumer.go:53 +0x12e
exit status 2
```

The engine has to known which middleware to use:

```go
// ...

bok.Use(middleware.Recoverer)
```

Now if an another task is produced and the worker run again:

```console
$ go run docs/main.go -run worker
2019/07/10 18:08:43 Received <Request task: <Task name=tasks.crawl id=01DFEA9QKBMPP6G587NS30YQKV, status=processing, published_at=2019-07-10 16:08:40 +0000 UTC>>
Panic: An unexpected error has happened
goroutine 23 [running]:
runtime/debug.Stack(0x28, 0x0, 0x0)
        /usr/local/Cellar/go/1.12.6/libexec/src/runtime/debug/stack.go:24 +0x9d
runtime/debug.PrintStack()
        /usr/local/Cellar/go/1.12.6/libexec/src/runtime/debug/stack.go:16 +0x22
github.com/thoas/bokchoy/middleware.Recoverer.func1.1(0xc000150200)
        /Users/thoas/Sites/golang/src/github.com/thoas/bokchoy/middleware/recoverer.go:20 +0x150
panic(0x12ff540, 0x13ea1f0)
        /usr/local/Cellar/go/1.12.6/libexec/src/runtime/panic.go:522 +0x1b5
main.main.func2(0xc000150200, 0x20, 0xc000046708)
        /Users/thoas/Sites/golang/src/github.com/thoas/bokchoy/docs/main.go:111 +0x98
github.com/thoas/bokchoy.HandlerFunc.Handle(0x1398de8, 0xc000150200, 0xc000150200, 0xc00010e120)
        /Users/thoas/Sites/golang/src/github.com/thoas/bokchoy/handler.go:8 +0x30
github.com/thoas/bokchoy/middleware.Recoverer.func1(0xc000150200, 0x0, 0x0)
        /Users/thoas/Sites/golang/src/github.com/thoas/bokchoy/middleware/recoverer.go:25 +0x7f
github.com/thoas/bokchoy.HandlerFunc.Handle(0xc00010e120, 0xc000150200, 0x1, 0x13f0c00)
        /Users/thoas/Sites/golang/src/github.com/thoas/bokchoy/handler.go:8 +0x30
github.com/thoas/bokchoy.(*consumer).handleTask.func1(0xc00011e090, 0xc000150200, 0xc000102090, 0xc00008a060)
        /Users/thoas/Sites/golang/src/github.com/thoas/bokchoy/consumer.go:54 +0x75
created by github.com/thoas/bokchoy.(*consumer).handleTask
        /Users/thoas/Sites/golang/src/github.com/thoas/bokchoy/consumer.go:53 +0x12e
```

It keeps its state and continue the workflow even after the panic, the task is marked as `failed`
like any handler which returns an error.

## Error reporting

All errors are now handled but how can we report them properly
in an error tracking software ([sentry](https://sentry.io) for example)?

There are three ways to report an error in Bokchoy, each option can be customized.

### Custom request logger

Bokchoy allows to implement a custom [LogFormatter](https://github.com/thoas/bokchoy/blob/master/middleware/logger.go)
which follow this interface:

```go
type LogFormatter interface {
	NewLogEntry(r *bokchoy.Request) LogEntry
}
```

or the default one can be used:

```go
bok.Use(middleware.DefaultLogger)
```

This middleware follows the Bokchoy request and also catch panic
if `middleware.Recoverer` is installed as well.

### Custom tracer

Internal errors and task errors can be catched by implementing
a custom [Tracer](https://github.com/thoas/bokchoy/blob/master/tracer.go) and provide it as an option
when initializing the engine.

The `Tracer` must follow the following interface:

```go
type Tracer interface {
	Log(context.Context, string, error)
}
```

### Catch failure events

Bokchoy has an internal event listener system to catch the state of the task during its lifecycle.

```go
queue.OnStartFunc(func(r *bokchoy.Request) error {
    // we update the context by adding a value
    *r = *r.WithContext(context.WithValue(r.Context(), "start_at", time.Now()))

    return nil
})

queue.OnCompleteFunc(func(r *bokchoy.Request) error {
    startAt, ok := r.Context().Value("start_at").(time.Time)
	if ok {
		fmt.Println(time.Since(startAt))
	}

    return nil
})

queue.OnSuccessFunc(func(r *bokchoy.Request) error {
    fmt.Println(r.Context().Value("start_at"))

    return nil
})

queue.OnFailureFunc(func(r *bokchoy.Request) error {
    fmt.Println(r.Context().Value("start_at"))
	fmt.Println("Error catched", r.Task.Error)

    return nil
})
```

The error can be catched in `OnFailureFunc` with an error reporting.

## Implementation

Both `middleware.Recoverer` and `middleware.DefaultLogger` are used:

```go
engine.Use(middleware.Recoverer)
engine.Use(middleware.DefaultLogger)
```

As we may need to trace each requests individually, we are adding `middleware.RequestID`
which attachs an unique ID to each requests.

This reference may be used to debug our application in production
using [Kibana](https://www.elastic.co/fr/products/kibana) to follow
the lifecycle of a request.

Bokchoy allows you to write your handler with two syntaxes:

```go
queue.HandleFunc(func(r *bokchoy.Request) error {
	// logic here

	return nil
})
```

or 

```go
type crawlHandler struct {
}

func (h *crawlHandler) Handle(r *bokchoy.Request) error {
	// logic here

	return nil
}

queue.Handle(&crawlHandler{})
```

The first syntax is useful for writing small handlers with
less logic, the second is used to store attributes in the
handler structure, we will use this one to store our HTTP client
instance.

A rewrite of the existing handler function:

```go
type crawlHandler struct {
}

func (h *crawlHandler) Handle(r *bokchoy.Request) error {
	res, err := json.Marshal(r.Task.Payload)
	if err != nil {
		return err
	}

	var crawl Crawl

	err = json.Unmarshal(res, &crawl)
	if err != nil {
		return err
	}

	log.Print("Received ", crawl)

	return nil
}
```

To start crawling, we need an HTTP client and an in-memory storage to store
urls already crawled to avoid crawling them twice:

```go
type crawlHandler struct {
	clt    *http.Client
	crawls map[string]int // Map<url, status_code>
}
```

To initialize a `crawlHandler` instance we declare a constructor which
creates a new `net/http` client with a custom timeout as parameter.

```go
func newCrawlHandler(timeout time.Duration) *crawlHandler {
	return &crawlHandler{
		clt: &http.Client{
			Timeout: time.Second * timeout,
			Transport: &http.Transport{
				Dial: (&net.Dialer{
					Timeout: timeout * time.Second,
				}).Dial,
				TLSHandshakeTimeout: timeout * time.Second,
			},
		},
		crawls: map[string]int{},
	}
}
```

The custom timeout is needed to force the `net/http` client to timeout
after a duration, Bokchoy already contains a timeout system to regain control
but it will leak the goroutine if the task doesn't have its proper timeout system.

[goquery](https://github.com/PuerkitoBio/goquery) will parse the response body and extract urls
from the document, urls will be filtered from a base url.

```go
// Crawls returns the crawls.
func (h *crawlHandler) Crawls() []string {
	crawls := make([]string, len(h.crawls))
	i := 0
	for url, _ := range h.crawls {
		crawls[i] = url
		i++
	}
	return crawls
}

// extractRelativeLinks extracts relative links from an net/http response with a base url.
// It returns links which only contain the base url to avoid crawling external links.
func (h *crawlHandler) extractRelativeLinks(baseURL string, res *http.Response) ([]string, error) {
	doc, err := goquery.NewDocumentFromResponse(res)
	if err != nil {
		return nil, err
	}

	links := h.filterLinks(baseURL, h.extractLinks(doc))
	crawls := h.Crawls()

	filteredLinks := []string{}

	for i := range links {
		if funk.InStrings(crawls, links[i]) {
			continue
		}

		filteredLinks = append(filteredLinks, links[i])
	}

	return filteredLinks, nil
}

// extractLinks extracts links from a goquery.Document.
func (h *crawlHandler) extractLinks(doc *goquery.Document) []string {
	foundUrls := []string{}
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		res, _ := s.Attr("href")
		foundUrls = append(foundUrls, res)
	})

	return foundUrls
}

// filterLinks filters links with a base url.
func (h *crawlHandler) filterLinks(baseURL string, links []string) []string {
	filteredLinks := []string{}

	for _, link := range links {
		if strings.HasPrefix(link, baseURL) {
			filteredLinks = append(filteredLinks, link)
		}

		if strings.HasPrefix(link, "/") {
			resolvedURL := fmt.Sprintf("%s%s", baseURL, link)
			filteredLinks = append(filteredLinks, resolvedURL)
		}
	}

	return filteredLinks
}
```

To keep the base URL between the main task and subtasks generated
by urls extracted from the document, we will pass it in the publish task.

```go
// Crawl defines a crawl.
type Crawl struct {
	BaseURL string `json:url`
	URL     string `json:"url"`
	Depth   int    `json:"depth"`
}

// Strings returns string representation of a crawl.
func (c Crawl) String() string {
	return fmt.Sprintf(
		"<Crawl url=%s depth=%d>",
		c.URL, c.Depth)
}
```

`BaseURL` attribute is added to the `Crawl` structure.

Last part is to update the `Handle` method to use `extractRelativeLinks` and publish
subtasks with `depth` decremented to stop the handler when it reaches zero:

```go
type crawlHandler struct {
	// ...
	queue  *bokchoy.Queue
}

func (h *crawlHandler) Handle(r *bokchoy.Request) error {
	res, err := json.Marshal(r.Task.Payload)
	if err != nil {
		return err
	}

	var crawl Crawl

	err = json.Unmarshal(res, &crawl)
	if err != nil {
		return err
	}

	log.Print("Received ", crawl)

	resp, err := h.clt.Get(crawl.URL)
	if err != nil {
		return err
	}

	log.Print("Crawled ", crawl.URL, " - [", resp.Status, "]")
	h.AddCrawl(crawl.URL, resp.StatusCode)

	if resp.StatusCode != 200 {
		return nil
	}

	defer resp.Body.Close()

	// depth is zero, the handler should stop
	if crawl.Depth == 0 {
		return nil
	}

	// extract relative links
	links, err := h.extractRelativeLinks(crawl.BaseURL, resp)
	if err != nil {
		return nil
	}

	for i := range links {
		// next crawls will still have the same base url
		// depth is decremented to stop the flow
		task, err := h.queue.Publish(r.Context(), &Crawl{
			URL:     links[i],
			BaseURL: crawl.BaseURL,
			Depth:   crawl.Depth - 1,
		})
		if err != nil {
			return err
		}

		log.Printf("%s published", task)
	}

	return nil
}
```

It's time to test the complete workflow by running the producer again:

```console
$ go run docs/main.go -run producer -url https://golang.org
```

Then the worker:

```console
$ go run docs/main.go -run worker
2019/07/13 08:56:24 Received <Crawl url=https://golang.org depth=1>
2019/07/13 08:56:25 Crawled https://golang.org - [200 OK]
2019/07/13 08:56:25 <Task name=tasks.crawl id=01DFN1WNSM3Z6KVT2606EPJXJ7, status=waiting, published_at=2019-07-13 06:56:25.396333 +0000 UTC> published
2019/07/13 08:56:25 <Task name=tasks.crawl id=01DFN1WNSMQ7CDZ4YW96BP2EPA, status=waiting, published_at=2019-07-13 06:56:25.396695 +0000 UTC> published
2019/07/13 08:56:25 <Task name=tasks.crawl id=01DFN1WNSMA1PZY59WWB2DMMA0, status=waiting, published_at=2019-07-13 06:56:25.396982 +0000 UTC> published
2019/07/13 08:56:25 <Task name=tasks.crawl id=01DFN1WNSNS69CT2BP1N0VQWZM, status=waiting, published_at=2019-07-13 06:56:25.397227 +0000 UTC> published
2019/07/13 08:56:25 <Task name=tasks.crawl id=01DFN1WNSNNFRHPFBEESPKNNZV, status=waiting, published_at=2019-07-13 06:56:25.397478 +0000 UTC> published
2019/07/13 08:56:25 <Task name=tasks.crawl id=01DFN1WNSN045VBNY0F45BNWFP, status=waiting, published_at=2019-07-13 06:56:25.397722 +0000 UTC> published
2019/07/13 08:56:25 <Task name=tasks.crawl id=01DFN1WNSNT80ARY5273DKG4JB, status=waiting, published_at=2019-07-13 06:56:25.397971 +0000 UTC> published
2019/07/13 08:56:25 <Task name=tasks.crawl id=01DFN1WNSP7GDNZ7DGFSPGC6ET, status=waiting, published_at=2019-07-13 06:56:25.398199 +0000 UTC> published
2019/07/13 08:56:25 <Task id=01DFN1WKNDN7ZAHHJDFVN8YSVV name=tasks.crawl payload=map[BaseURL:https://golang.org depth:1 url:https://golang.org]> - succeeded - result: (empty) in 445.079628ms
2019/07/13 08:56:25 Received <Crawl url=https://golang.org/doc/tos.html depth=0>
2019/07/13 08:56:25 Crawled https://golang.org/doc/tos.html - [200 OK]
2019/07/13 08:56:25 <Task id=01DFN1WNSP7GDNZ7DGFSPGC6ET name=tasks.crawl payload=map[BaseURL:https://golang.org depth:0 url:https://golang.org/doc/tos.html]> - succeeded - result: (empty) in 137.121649ms
2019/07/13 08:56:25 Received <Crawl url=https://golang.org/doc/copyright.html depth=0>
2019/07/13 08:56:25 Crawled https://golang.org/doc/copyright.html - [200 OK]
2019/07/13 08:56:25 <Task id=01DFN1WNSNT80ARY5273DKG4JB name=tasks.crawl payload=map[BaseURL:https://golang.org depth:0 url:https://golang.org/doc/copyright.html]> - succeeded - result: (empty) in 294.807462ms
2019/07/13 08:56:25 Received <Crawl url=https://golang.org/dl depth=0>
2019/07/13 08:56:26 Crawled https://golang.org/dl - [200 OK]
2019/07/13 08:56:26 <Task id=01DFN1WNSN045VBNY0F45BNWFP name=tasks.crawl payload=map[BaseURL:https://golang.org depth:0 url:https://golang.org/dl]> - succeeded - result: (empty) in 1.051028215s
2019/07/13 08:56:26 Received <Crawl url=https://golang.org/blog depth=0>
2019/07/13 08:56:27 Crawled https://golang.org/blog - [200 OK]
2019/07/13 08:56:27 <Task id=01DFN1WNSNNFRHPFBEESPKNNZV name=tasks.crawl payload=map[BaseURL:https://golang.org depth:0 url:https://golang.org/blog]> - succeeded - result: (empty) in 813.442227ms
2019/07/13 08:56:27 Received <Crawl url=https://golang.org/help depth=0>
2019/07/13 08:56:28 Crawled https://golang.org/help - [200 OK]
2019/07/13 08:56:28 <Task id=01DFN1WNSNS69CT2BP1N0VQWZM name=tasks.crawl payload=map[BaseURL:https://golang.org depth:0 url:https://golang.org/help]> - succeeded - result: (empty) in 721.972494ms
2019/07/13 08:56:28 Received <Crawl url=https://golang.org/project depth=0>
2019/07/13 08:56:28 Crawled https://golang.org/project - [200 OK]
2019/07/13 08:56:28 <Task id=01DFN1WNSMA1PZY59WWB2DMMA0 name=tasks.crawl payload=map[BaseURL:https://golang.org depth:0 url:https://golang.org/project]> - succeeded - result: (empty) in 411.728612ms
2019/07/13 08:56:28 Received <Crawl url=https://golang.org/pkg depth=0>
2019/07/13 08:56:29 Crawled https://golang.org/pkg - [200 OK]
2019/07/13 08:56:29 <Task id=01DFN1WNSMQ7CDZ4YW96BP2EPA name=tasks.crawl payload=map[BaseURL:https://golang.org depth:0 url:https://golang.org/pkg]> - succeeded - result: (empty) in 408.950376ms
2019/07/13 08:56:29 Received <Crawl url=https://golang.org/doc depth=0>
2019/07/13 08:56:29 Crawled https://golang.org/doc - [200 OK]
2019/07/13 08:56:29 <Task id=01DFN1WNSM3Z6KVT2606EPJXJ7 name=tasks.crawl payload=map[BaseURL:https://golang.org depth:0 url:https://golang.org/doc]> - succeeded - result: (empty) in 367.4162ms
```

It works like a charm, still a bit slow to crawl and it can be even slower with a higher `depth` value.

## Concurrency

Bokchoy allows you to spawn easily multiple goroutines per queue or globally on the engine.

We will add a new `concurrency` flag to the top to configure the number of worker set for this queue:

```go
var concurrency int
flag.IntVar(&concurrency, "concurrency", 1, "number of workers")
```

The line is updated:

```go
queue.Handle(&crawlHandler{})
```

as follow:

```go
queue.Handle(&crawlHandler{}, bokchoy.WithConcurrency(concurrency))
```

Concurrency comes with a potential race condition issue,
we use [sync](https://golang.org/pkg/sync/) package to avoid it:

```go

type crawlHandler struct {
	// ...
	mu     sync.RWMutex
}

// AddCrawl adds a new crawl to the storage.
func (h *crawlHandler) AddCrawl(url string, statusCode int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.crawls[url] = statusCode
}

// Crawls returns the crawls.
func (h *crawlHandler) Crawls() []string {
	h.mu.RLock()
	crawls := make([]string, len(h.crawls))
	i := 0
	for url, _ := range h.crawls {
		crawls[i] = url
		i++
	}
	h.mu.RUnlock()

	return crawls
}
```

## Conclusion

It has been a long tour, if you have reach to the bottom you belong to the brave ☺.

There are multiple others features ([timeout](https://github.com/thoas/bokchoy#timeout),
 [custom logger](https://github.com/thoas/bokchoy#custom-logger),
 [delayed task](https://github.com/thoas/bokchoy#delayed-task), ...)
which are not described in this tour, if you are curious enough
go check the [README](https://github.com/thoas/bokchoy) of the project.


* Ping me on twitter [@thoas](https://twitter.com/thoas)
* Fork the [project](https://github.com/thoas/bokchoy)
* Fix [bugs](https://github.com/thoas/bokchoy/issues)

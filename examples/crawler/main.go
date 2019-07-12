package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/thoas/bokchoy"
	"github.com/thoas/bokchoy/logging"
	"github.com/thoas/bokchoy/middleware"
	"github.com/thoas/go-funk"
)

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

func newCrawlHandler(queue *bokchoy.Queue, timeout time.Duration) *crawlHandler {
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
		queue:  queue,
	}
}

type crawlHandler struct {
	clt    *http.Client
	crawls map[string]int
	mu     sync.RWMutex
	queue  *bokchoy.Queue
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

	switch run {
	case "producer":
		task, err := queue.Publish(ctx, &Crawl{
			URL:     url,
			BaseURL: url,
			Depth:   depth,
		})
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("%s published", task)
	case "worker":
		h := newCrawlHandler(queue, time.Duration(timeout))
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

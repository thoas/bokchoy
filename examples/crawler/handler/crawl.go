package handler

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/thoas/go-funk"

	"github.com/thoas/bokchoy"
	"github.com/thoas/bokchoy/examples/crawler/parser"
	"github.com/thoas/bokchoy/examples/crawler/task"
)

// NewCrawlHandler initializes a new CrawlHandler instance.
func NewCrawlHandler(queue *bokchoy.Queue, parser parser.Parser, timeout time.Duration) *CrawlHandler {
	return &CrawlHandler{
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
		parser: parser,
	}
}

type CrawlHandler struct {
	clt    *http.Client
	crawls map[string]int
	mu     sync.RWMutex
	queue  *bokchoy.Queue
	parser parser.Parser
}

// AddCrawl adds a new crawl to the storage.
func (h *CrawlHandler) AddCrawl(url string, statusCode int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.crawls[url] = statusCode
}

// Crawls returns the crawls.
func (h *CrawlHandler) Crawls() []string {
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

// Crawl publishes a new task to crawl an url with its base URL.
func (h *CrawlHandler) Crawl(ctx context.Context, baseURL string, url string, depth int) (*bokchoy.Task, error) {
	url = strings.TrimSuffix(url, "/")
	baseURL = strings.TrimSuffix(baseURL, "/")

	task, err := h.queue.Publish(ctx, &task.Crawl{
		URL:     url,
		BaseURL: baseURL,
		Depth:   depth,
	})
	if err != nil {
		return nil, err
	}
	return task, nil
}

func (h *CrawlHandler) extractLinks(baseURL string, res *http.Response) ([]string, error) {
	links, err := h.parser.ExtractLinks(baseURL, res.Body)
	if err != nil {
		return nil, err
	}

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

// Handle handles a bokchoy.Request.
func (h *CrawlHandler) Handle(r *bokchoy.Request) error {
	res, err := json.Marshal(r.Task.Payload)
	if err != nil {
		return err
	}

	var crawl task.Crawl

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
	links, err := h.extractLinks(crawl.BaseURL, resp)
	if err != nil {
		return nil
	}

	for i := range links {
		// next crawls will still have the same base url
		// depth is decremented to stop the flow
		task, err := h.Crawl(r.Context(), crawl.BaseURL, links[i], crawl.Depth-1)
		if err != nil {
			return err
		}

		log.Printf("%s published", task)
	}

	return nil
}

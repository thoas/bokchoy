package task

import "fmt"

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

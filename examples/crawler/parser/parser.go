package parser

import (
	"fmt"
	"io"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Parser is a common interface to parse a document.
type Parser interface {
	ExtractLinks(string, io.Reader) ([]string, error)
}

// DocumentParser is an HTML document parser.
type DocumentParser struct {
}

// ExtractLinks extracts relative links from an net/http response with a base url.
// It returns links which only contain the base url to avoid crawling external links.
func (p *DocumentParser) ExtractLinks(baseURL string, body io.Reader) ([]string, error) {
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, err
	}

	return filterLinks(baseURL, extractLinks(doc)), nil
}

// extractLinks extracts links from a goquery.Document.
func extractLinks(doc *goquery.Document) []string {
	foundUrls := []string{}
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		res, _ := s.Attr("href")
		foundUrls = append(foundUrls, res)
	})

	return foundUrls
}

// filterLinks filters links with a base url.
func filterLinks(baseURL string, links []string) []string {
	filteredLinks := []string{}

	for _, link := range links {
		link = strings.TrimSuffix(link, "/")

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

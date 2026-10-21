package crawler

import (
	"errors"
	"net/http"
	"net/url"
)

type crawler struct {
	client     *http.Client
	rootURL    *url.URL
	depth      uint
	visited    map[string]bool
	pages      []Page
	indentJSON uint
}

func newCrawler(opts Options) (*crawler, error) {
	if opts.HTTPClient == nil {
		return nil, errors.New("http client is required")
	}

	normURL, err := normalizeURL(opts.URL)
	if err != nil {
		return nil, err
	}

	return &crawler{
		rootURL:    normURL,
		client:     opts.HTTPClient,
		depth:      opts.Depth,
		indentJSON: opts.IndentJSON,
		visited:    map[string]bool{},
	}, nil
}

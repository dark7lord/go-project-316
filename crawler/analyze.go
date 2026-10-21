// Package crawler implements website structure analysis.
package crawler

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Options configures a crawl run.
type Options struct {
	URL         string
	Depth       uint
	Retries     uint
	Delay       time.Duration
	Timeout     time.Duration
	UserAgent   string
	Concurrency uint
	IndentJSON  uint
	HTTPClient  *http.Client
}

// Analyze crawls a website and returns a report about its structure.
func Analyze(ctx context.Context, opts Options) (data []byte, err error) {
	if opts.HTTPClient == nil {
		return nil, errors.New("http client is required")
	}

	normURL, err := normalizeURL(opts.URL)
	if err != nil {
		return nil, err
	}

	_ = ctx

	crw := crawler{
		rootURL: normURL,
		client:  opts.HTTPClient,
		depth:   opts.Depth,
		visited: map[string]bool{},
	}

	page := crw.fetchPage(normURL)
	crw.pages = append(crw.pages, page)

	return report(crw, opts.IndentJSON)
}

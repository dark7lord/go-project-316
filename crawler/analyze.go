// Package crawler implements website structure analysis.
package crawler

import (
	"context"
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
	_ = ctx

	crw, err := newCrawler(opts)
	if err != nil {
		return nil, err
	}

	page := crw.fetchPage(crw.rootURL)
	crw.pages = append(crw.pages, page)

	return crw.report()
}

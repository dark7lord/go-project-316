// Package crawler implements website structure analysis.
package crawler

import (
	"context"
	"net/http"
	"time"
)

// Options configures a crawl run.
type Options struct {
	URL     string
	Depth   uint
	Retries uint
	// Delay is the minimal pause between the starts of two requests,
	// shared by the whole crawl. Zero or negative means no limit.
	Delay time.Duration
	// Timeout is the maximum duration of one HTTP attempt, including reading
	// its response body. Zero preserves the HTTP client's configured timeout.
	Timeout     time.Duration
	UserAgent   string
	Concurrency uint
	IndentJSON  uint
	HTTPClient  *http.Client
}

// Analyze crawls a website and returns a report about its structure.
func Analyze(ctx context.Context, opts Options) (data []byte, err error) {
	crw, err := newCrawler(opts)
	if err != nil {
		return nil, err
	}

	crw.crawl(ctx)

	return crw.report()
}

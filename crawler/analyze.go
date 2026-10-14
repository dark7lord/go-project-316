// Package crawler implements website structure analysis.
package crawler

import (
	"context"
	"encoding/json"
	"fmt"
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

// Page is a single crawled page in the report.
type Page struct {
	URL    string `json:"url"`
	Depth  uint   `json:"depth"`
	Status int    `json:"http_status"`
	Error  string `json:"error,omitempty"`
}

// Result is the crawl report.
type Result struct {
	RootURL     string    `json:"root_url"`
	Depth       uint      `json:"depth"`
	GeneratedAt time.Time `json:"generated_at"`
	Pages       []Page    `json:"pages"`
}

func report(opts Options) Result {
	return Result{
		RootURL:     opts.URL,
		Depth:       opts.Depth,
		GeneratedAt: time.Now(),
	}
}

// Analyze crawls a website and returns a report about its structure.
func Analyze(ctx context.Context, opts Options) (data []byte, err error) {
	result := report(opts)
	page := Page{URL: opts.URL}

	defer func() {
		if r := recover(); r != nil {
			page.Error = fmt.Sprint("panic: ", r)
			result.Pages = []Page{page}
			data, err = json.MarshalIndent(result, "", "  ")
		}
	}()
	_ = ctx

	resp, err := opts.HTTPClient.Get(opts.URL)
	if err != nil {
		page.Error = err.Error()
	} else {
		page.Status = resp.StatusCode
		_ = resp.Body.Close()
	}

	result.Pages = []Page{page}

	return json.MarshalIndent(result, "", "  ")
}

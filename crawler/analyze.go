// Package crawler implements website structure analysis.
package crawler

import (
	"context"
	"encoding/json"
	"errors"
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
	URL        string `json:"url"`
	Depth      uint   `json:"depth"`
	HTTPStatus int    `json:"http_status"`
	Status     string `json:"status"`
	Error      string `json:"error"`
}

// Result is the crawl report.
type Result struct {
	RootURL     string    `json:"root_url"`
	Depth       uint      `json:"depth"`
	GeneratedAt time.Time `json:"generated_at"`
	Pages       []Page    `json:"pages"`
}

type crawler struct {
	client  *http.Client
	visited map[string]bool
}

const (
	statusOK    = "ok"
	statusError = "error"
)

func (crw *crawler) fetch(url string) (page Page) {
	page = Page{URL: url}

	defer func() {
		if r := recover(); r != nil {
			page.Status = statusError
			page.Error = fmt.Sprint("panic: ", r)
		}
	}()

	crw.visited[url] = true
	resp, err := crw.client.Get(url)
	if err != nil {
		page.Status = statusError
		page.Error = err.Error()

		return page
	}
	defer func() { _ = resp.Body.Close() }()

	page.HTTPStatus = resp.StatusCode
	page.Status = statusOK
	if resp.StatusCode >= 400 {
		page.Status = statusError
	}

	return page
}

// Analyze crawls a website and returns a report about its structure.
func Analyze(ctx context.Context, opts Options) (data []byte, err error) {
	rootURL, err := normalizeURL(opts.URL)
	if err != nil {
		return nil, err
	}

	if opts.HTTPClient == nil {
		return nil, errors.New("http client is required")
	}

	result := Result{
		RootURL:     rootURL,
		Depth:       opts.Depth,
		GeneratedAt: time.Now(),
	}

	_ = ctx

	crw := crawler{
		client:  opts.HTTPClient,
		visited: map[string]bool{},
	}

	page := crw.fetch(rootURL)

	result.Pages = []Page{page}

	return json.MarshalIndent(result, "", "  ")
}

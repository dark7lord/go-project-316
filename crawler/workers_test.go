package crawler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"code/crawler"
)

func TestCrawlWorkerLimit(t *testing.T) {
	const (
		concurrency = 3
		pageCount   = 6
	)

	var mu sync.Mutex
	var active int
	var maxActive int

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		mu.Unlock()

		defer func() {
			mu.Lock()
			active--
			mu.Unlock()
		}()

		time.Sleep(250 * time.Millisecond)

		if r.URL.Path == "/" {
			for i := range pageCount {
				_, _ = fmt.Fprintf(w, `<a href="/page-%d">page</a>`, i)
			}
		}
	})
	server := newTestServer(t, handler)

	report := analyzeReport(t, crawler.Options{
		URL:         server.URL,
		HTTPClient:  server.Client(),
		Depth:       2,
		Concurrency: concurrency,
	})
	require.Len(t, report.Pages, pageCount+1)

	mu.Lock()
	observedMax := maxActive
	mu.Unlock()

	require.LessOrEqual(t, observedMax, concurrency)
	require.Greater(t, observedMax, 1)
}

func TestCrawlConcurrencyDeterminism(t *testing.T) {
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = fmt.Fprint(w, `<a href="/a">a</a><a href="/b">b</a>`)
		case "/a":
			_, _ = fmt.Fprint(w, `<a href="/c">c</a>`)
		case "/b":
			_, _ = fmt.Fprint(w, `<a href="/c">c</a>`)
		}
	}))

	wantDepths := map[string]uint{"": 0, "/a": 1, "/b": 1, "/c": 2}
	wantURLs := []string{server.URL, server.URL + "/a", server.URL + "/b", server.URL + "/c"}

	for _, concurrency := range []uint{0, 1, 4} {
		t.Run(fmt.Sprintf("concurrency %d", concurrency), func(t *testing.T) {
			report := analyzeReport(t, crawler.Options{
				URL:         server.URL,
				HTTPClient:  server.Client(),
				Depth:       3,
				Concurrency: concurrency,
			})

			require.Equal(t, wantDepths, pageDepthsFromReport(t, report))
			require.Equal(t, wantURLs, reportURLs(report))
		})
	}
}

func TestCrawlCancellationWithWorkers(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var pageRequests [4]atomic.Int32
	var active atomic.Int32
	canceledWithConcurrentRequests := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = fmt.Fprint(w, `
				<a href="/a"> a </a>
				<a href="/b"> b </a>
				<a href="/c"> c </a>
				<a href="/d"> d </a>
			`)
		case "/a", "/b", "/c", "/d":
			letter := r.URL.Path[1]
			idx := int(letter - 'a')
			if pageRequests[idx].Add(1) > 1 {
				current := active.Add(1)
				defer active.Add(-1)
				if current == 2 {
					close(canceledWithConcurrentRequests)
					cancel()
				}
				<-r.Context().Done()
			}
		}
	})
	server := newTestServer(t, handler)

	result := make(chan []byte, 1)
	errs := make(chan error, 1)
	go func() {
		data, err := crawler.Analyze(ctx, crawler.Options{
			URL:         server.URL,
			HTTPClient:  server.Client(),
			Depth:       2,
			Concurrency: 4,
		})
		result <- data
		errs <- err
	}()

	select {
	case <-canceledWithConcurrentRequests:
	case <-time.After(3 * time.Second):
		t.Fatal("multiple page requests did not run concurrently")
	}

	var data []byte
	select {
	case data = <-result:
	case <-time.After(3 * time.Second):
		t.Fatal("crawl did not stop after cancellation")
	}
	require.NoError(t, <-errs)

	var report crawler.Report
	require.NoError(t, json.Unmarshal(data, &report))
	require.NotEmpty(t, report.Pages)
	require.Equal(t, server.URL, report.Pages[0].URL)
}

func TestCrawlDelayIsGlobalWithWorkers(t *testing.T) {
	const (
		delay     = 20 * time.Millisecond
		pageCount = 6
	)

	starts := make(chan time.Time, pageCount*2+1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		starts <- time.Now()
		if r.URL.Path == "/" {
			for i := range pageCount {
				_, _ = fmt.Fprintf(w, `<a href="/page-%d">page</a>`, i)
			}
		}
	})
	server := newTestServer(t, handler)

	_, err := crawler.Analyze(t.Context(), crawler.Options{
		URL:         server.URL,
		HTTPClient:  server.Client(),
		Depth:       2,
		Concurrency: 4,
		Delay:       delay,
	})
	require.NoError(t, err)

	var allStarts []time.Time
	for range pageCount*2 + 1 {
		allStarts = append(allStarts, <-starts)
	}
	require.GreaterOrEqual(t, allStarts[len(allStarts)-1].Sub(allStarts[0]), time.Duration(len(allStarts)-1)*delay)
}

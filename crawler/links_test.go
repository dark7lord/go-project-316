package crawler_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"code/crawler"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func brokenLink(url string, statusCode int) crawler.BrokenLink {
	return crawler.BrokenLink{
		URL:        url,
		StatusCode: statusCode,
		Error:      http.StatusText(statusCode),
	}
}

type linkRoute struct {
	status int
	body   string
}

func newLinkSite(t *testing.T, root string, routes map[string]linkRoute) (*httptest.Server, func(string) int32) {
	t.Helper()

	var mu sync.Mutex
	requests := map[string]int32{}

	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests[r.URL.Path]++
		mu.Unlock()

		if r.URL.Path == "/" {
			_, _ = fmt.Fprint(w, root)

			return
		}

		route, ok := routes[r.URL.Path]
		if !ok {
			http.NotFound(w, r)

			return
		}

		status := route.status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, route.body)
	}))

	return server, func(path string) int32 {
		mu.Lock()
		defer mu.Unlock()

		return requests[path]
	}
}

func TestAnalyzeBrokenLinks(t *testing.T) {
	type brokenExpectation struct {
		path   string
		status int
	}

	tests := []struct {
		name           string
		root           string
		routes         map[string]linkRoute
		depth          uint
		retries        uint
		wantPages      int
		wantBrokenPage int
		wantBroken     []brokenExpectation
		wantRequests   map[string]int32
	}{
		{
			name:         "same host link returns 404",
			root:         `<a href="/missing">missing</a>`,
			depth:        1,
			wantPages:    1,
			wantBroken:   []brokenExpectation{{path: "/missing", status: http.StatusNotFound}},
			wantRequests: map[string]int32{"/missing": 1},
		},
		{
			name:         "depth zero still checks root links",
			root:         `<a href="/missing">missing</a>`,
			wantPages:    1,
			wantBroken:   []brokenExpectation{{path: "/missing", status: http.StatusNotFound}},
			wantRequests: map[string]int32{"/missing": 1},
		},
		{
			name:         "depth one checks children without crawling them",
			root:         `<a href="/child">child</a><a href="/missing">missing</a>`,
			routes:       map[string]linkRoute{"/child": {body: "<title>Child</title>"}},
			depth:        1,
			wantPages:    1,
			wantBroken:   []brokenExpectation{{path: "/missing", status: http.StatusNotFound}},
			wantRequests: map[string]int32{"/child": 1, "/missing": 1},
		},
		{
			name:           "depth two checks links on crawled children",
			root:           `<a href="/child">child</a>`,
			routes:         map[string]linkRoute{"/child": {body: `<a href="/missing">missing</a>`}},
			depth:          2,
			wantPages:      2,
			wantBrokenPage: 1,
			wantBroken:     []brokenExpectation{{path: "/missing", status: http.StatusNotFound}},
			wantRequests:   map[string]int32{"/missing": 1},
		},
		{
			name:         "fragments are checked once",
			root:         `<a href="/missing#first">first</a><a href="/missing#second">second</a>`,
			depth:        1,
			wantPages:    1,
			wantBroken:   []brokenExpectation{{path: "/missing", status: http.StatusNotFound}},
			wantRequests: map[string]int32{"/missing": 1},
		},
		{
			name:      "trailing slash is a separate url",
			root:      `<a href="/missing">without slash</a><a href="/missing/">with slash</a>`,
			depth:     1,
			wantPages: 1,
			wantBroken: []brokenExpectation{
				{path: "/missing", status: http.StatusNotFound},
				{path: "/missing/", status: http.StatusNotFound},
			},
			wantRequests: map[string]int32{"/missing": 1, "/missing/": 1},
		},
		{
			name:         "retries report the final status",
			root:         `<a href="/flaky">retry me</a>`,
			routes:       map[string]linkRoute{"/flaky": {status: http.StatusServiceUnavailable}},
			depth:        1,
			retries:      2,
			wantPages:    1,
			wantBroken:   []brokenExpectation{{path: "/flaky", status: http.StatusServiceUnavailable}},
			wantRequests: map[string]int32{"/flaky": 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, requests := newLinkSite(t, tt.root, tt.routes)

			report := analyzeReport(t, crawler.Options{
				URL:        server.URL,
				Depth:      tt.depth,
				Retries:    tt.retries,
				HTTPClient: server.Client(),
			})
			require.Len(t, report.Pages, tt.wantPages)

			want := make([]crawler.BrokenLink, len(tt.wantBroken))
			for i, link := range tt.wantBroken {
				want[i] = brokenLink(server.URL+link.path, link.status)
			}
			require.Equal(t, want, report.Pages[tt.wantBrokenPage].BrokenLinks)

			for path, count := range tt.wantRequests {
				require.Equal(t, count, requests(path), path)
			}
		})
	}
}

func TestAnalyzeExternalBrokenLinkIsCheckedButNotCrawled(t *testing.T) {
	var externalRequests atomic.Int32
	externalServer := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		externalRequests.Add(1)
		http.NotFound(w, r)
	}))

	root := fmt.Sprintf(
		`<a href="/child">internal</a><a href="%s/outside">external</a>`,
		externalServer.URL,
	)
	server, _ := newLinkSite(t, root, map[string]linkRoute{"/child": {body: `<title>Child</title>`}})

	report := analyzeReport(t, crawler.Options{
		URL:        server.URL,
		Depth:      2,
		HTTPClient: server.Client(),
	})
	require.Len(t, report.Pages, 2)
	require.Equal(t, []crawler.BrokenLink{
		brokenLink(externalServer.URL+"/outside", http.StatusNotFound),
	}, report.Pages[0].BrokenLinks)
	require.Equal(t, int32(1), externalRequests.Load())
	require.NotContains(t, reportURLs(report), externalServer.URL+"/outside")
}

func TestAnalyzeBrokenLinkNetworkError(t *testing.T) {
	server, _ := newLinkSite(t, `<a href="/unreachable">dead</a>`, nil)

	client := server.Client()
	client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/unreachable" {
			return nil, errors.New("network error")
		}

		return http.DefaultTransport.RoundTrip(req)
	})

	report := analyzeReport(t, crawler.Options{
		URL:        server.URL,
		Depth:      1,
		HTTPClient: client,
	})
	require.Len(t, report.Pages, 1)
	require.Len(t, report.Pages[0].BrokenLinks, 1)

	link := report.Pages[0].BrokenLinks[0]
	require.Equal(t, server.URL+"/unreachable", link.URL)
	require.Zero(t, link.StatusCode)
	require.Contains(t, link.Error, "network error")
}

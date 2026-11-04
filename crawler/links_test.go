package crawler_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"code/crawler"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestBrokenLinks(t *testing.T) {
	server := setupTestServer(t)

	page := firstPage(t, crawler.Options{
		URL:        server.URL + "/",
		Depth:      1,
		HTTPClient: server.Client(),
	})

	want := []crawler.BrokenLink{{
		URL:        server.URL + "/not-found",
		StatusCode: http.StatusNotFound,
		Error:      http.StatusText(http.StatusNotFound),
	}}
	require.Equal(t, want, page.BrokenLinks)
}

func TestExternalBrokenLinksAreCheckedButNotCrawled(t *testing.T) {
	var externalRequests atomic.Int32
	externalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		externalRequests.Add(1)
		http.NotFound(w, r)
	}))
	t.Cleanup(externalServer.Close)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			_, _ = fmt.Fprintf(w, `<a href="/child">internal</a><a href="%s/outside">external</a>`, externalServer.URL)
			return
		}
		_, _ = w.Write([]byte(`<title>Child</title>`))
	}))
	t.Cleanup(server.Close)

	data, err := crawler.Analyze(t.Context(), crawler.Options{
		URL:        server.URL,
		Depth:      2,
		HTTPClient: server.Client(),
	})
	require.NoError(t, err)

	var report crawler.Report
	require.NoError(t, json.Unmarshal(data, &report))
	require.Len(t, report.Pages, 2)
	require.Equal(t, []crawler.BrokenLink{{
		URL:        externalServer.URL + "/outside",
		StatusCode: http.StatusNotFound,
		Error:      http.StatusText(http.StatusNotFound),
	}}, report.Pages[0].BrokenLinks)
	require.Equal(t, int32(1), externalRequests.Load())
	require.NotContains(t, []string{report.Pages[0].URL, report.Pages[1].URL}, externalServer.URL+"/outside")
}

func TestBrokenLinksWithFragmentsAreCheckedOnce(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			_, _ = w.Write([]byte(`
				<a href="/missing#first">first</a>
				<a href="/missing#second">second</a>
			`))

			return
		}

		requests.Add(1)
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)

	page := firstPage(t, crawler.Options{
		URL:        server.URL,
		Depth:      1,
		HTTPClient: server.Client(),
	})

	require.Equal(t, []crawler.BrokenLink{{
		URL:        server.URL + "/missing",
		StatusCode: http.StatusNotFound,
		Error:      http.StatusText(http.StatusNotFound),
	}}, page.BrokenLinks)
	require.Equal(t, int32(1), requests.Load())
}

func TestBrokenLinksWithTrailingSlashAreCheckedSeparately(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			_, _ = w.Write([]byte(`
				<a href="/missing">without slash</a>
				<a href="/missing/">with slash</a>
			`))

			return
		}

		requests.Add(1)
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)

	page := firstPage(t, crawler.Options{
		URL:        server.URL,
		Depth:      1,
		HTTPClient: server.Client(),
	})

	require.Equal(t, []crawler.BrokenLink{
		{
			URL:        server.URL + "/missing",
			StatusCode: http.StatusNotFound,
			Error:      http.StatusText(http.StatusNotFound),
		},
		{
			URL:        server.URL + "/missing/",
			StatusCode: http.StatusNotFound,
			Error:      http.StatusText(http.StatusNotFound),
		},
	}, page.BrokenLinks)
	require.Equal(t, int32(2), requests.Load())
}

func TestBrokenLinksNetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			_, _ = w.Write([]byte(`<a href="/unreachable">dead</a>`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)

	client := server.Client()
	client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/unreachable" {
			return nil, errors.New("network error")
		}

		return http.DefaultTransport.RoundTrip(req)
	})

	page := firstPage(t, crawler.Options{
		URL:        server.URL,
		Depth:      1,
		HTTPClient: client,
	})

	require.Len(t, page.BrokenLinks, 1)
	require.Equal(t, server.URL+"/unreachable", page.BrokenLinks[0].URL)
	require.Zero(t, page.BrokenLinks[0].StatusCode)
	require.Contains(t, page.BrokenLinks[0].Error, "network error")
}

func TestBrokenLinkReportContainsStatusAfterRetries(t *testing.T) {
	var failedLinkRequests atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/flaky" {
			failedLinkRequests.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		_, _ = w.Write([]byte(`<a href="/flaky">retry me</a>`))
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	data, err := crawler.Analyze(t.Context(), crawler.Options{
		URL:        server.URL,
		Depth:      1,
		HTTPClient: server.Client(),
		Retries:    2,
	})
	require.NoError(t, err)

	var report crawler.Report
	require.NoError(t, json.Unmarshal(data, &report))
	require.NotEmpty(t, report.Pages)
	require.Equal(t, []crawler.BrokenLink{{
		URL:        server.URL + "/flaky",
		StatusCode: http.StatusServiceUnavailable,
		Error:      http.StatusText(http.StatusServiceUnavailable),
	}}, report.Pages[0].BrokenLinks)
	require.Equal(t, int32(3), failedLinkRequests.Load())
}

func TestBrokenLinksContextCanceled(t *testing.T) {
	server, ctx := newCancelServer(t)

	opts := crawler.Options{URL: server.URL, Depth: 1, HTTPClient: server.Client()}

	data, err := crawler.Analyze(ctx, opts)
	require.NoError(t, err)

	var report crawler.Report
	require.NoError(t, json.Unmarshal(data, &report))
	require.NotEmpty(t, report.Pages)
	page := report.Pages[0]
	require.Equal(t, "ok", page.Status)
	require.Empty(t, page.BrokenLinks)
	require.ErrorIs(t, ctx.Err(), context.Canceled)
}

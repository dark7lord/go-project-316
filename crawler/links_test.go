package crawler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"code/crawler"
)

func TestBrokenLinks(t *testing.T) {
	server := setupTestServer(t)

	page := firstPage(t, crawler.Options{
		URL:        server.URL + "/",
		HTTPClient: server.Client(),
	})

	want := []crawler.BrokenLink{{
		URL:        server.URL + "/not-found",
		StatusCode: http.StatusNotFound,
		Error:      http.StatusText(http.StatusNotFound),
	}}
	require.Equal(t, want, page.BrokenLinks)
}

func TestBrokenLinksNetworkError(t *testing.T) {
	server := setupTestServer(t)

	page := firstPage(t, crawler.Options{
		URL:        server.URL + "/dead-link.html",
		HTTPClient: server.Client(),
	})

	require.Len(t, page.BrokenLinks, 1)
	require.Equal(t, "http://127.0.0.1:1/", page.BrokenLinks[0].URL)
	require.Zero(t, page.BrokenLinks[0].StatusCode)
	require.NotEmpty(t, page.BrokenLinks[0].Error)
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
		HTTPClient: server.Client(),
		Retries:    2,
	})
	require.NoError(t, err)

	var report crawler.Report
	require.NoError(t, json.Unmarshal(data, &report))
	require.Len(t, report.Pages, 1)
	require.Equal(t, []crawler.BrokenLink{{
		URL:        server.URL + "/flaky",
		StatusCode: http.StatusServiceUnavailable,
		Error:      http.StatusText(http.StatusServiceUnavailable),
	}}, report.Pages[0].BrokenLinks)
	require.Equal(t, int32(3), failedLinkRequests.Load())
}

func TestBrokenLinksContextCanceled(t *testing.T) {
	server, ctx := newCancelServer(t)

	opts := crawler.Options{URL: server.URL, HTTPClient: server.Client()}

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

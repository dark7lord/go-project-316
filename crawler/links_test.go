package crawler_test

import (
	"context"
	"encoding/json"
	"net/http"
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

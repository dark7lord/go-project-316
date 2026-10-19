package crawler_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"code/crawler"
)

func TestBrokenLinks(t *testing.T) {
	server := setupTestServer(t)

	page := firstPage(t, crawler.Options{URL: server.URL + "/", HTTPClient: server.Client()})

	want := []crawler.Link{{
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

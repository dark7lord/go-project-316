package crawler_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"code/crawler"
)

func TestAnalyze(t *testing.T) {
	server := setupTestServer(t)

	tests := []struct {
		name       string
		path       string
		statusCode int
		statusText string
	}{
		{
			name:       "success",
			path:       "/",
			statusCode: http.StatusOK,
			statusText: "ok",
		},
		{
			name:       "not found",
			path:       "/missing",
			statusCode: http.StatusNotFound,
			statusText: "error",
		},
		{
			name:       "server error",
			path:       "/server-error",
			statusCode: http.StatusInternalServerError,
			statusText: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := firstPage(t, crawler.Options{
				URL:        server.URL + tt.path,
				HTTPClient: server.Client(),
			})

			require.Equal(t, tt.statusCode, page.HTTPStatus)
			require.Equal(t, tt.statusText, page.Status)
			require.Empty(t, page.Error)
		})
	}
}

func TestAnalyzeNetworkError(t *testing.T) {
	server := setupTestServer(t)
	url := server.URL
	client := server.Client()
	server.Close()

	page := firstPage(t, crawler.Options{URL: url, HTTPClient: client})

	require.Zero(t, page.HTTPStatus)
	require.Equal(t, "error", page.Status)
	require.NotEmpty(t, page.Error)
}

func TestAnalyzeTimeout(t *testing.T) {
	server := setupTestServer(t)
	client := server.Client()
	url := server.URL + "/slow"
	client.Timeout = 50 * time.Millisecond

	page := firstPage(t, crawler.Options{URL: url, HTTPClient: client})

	require.Equal(t, "error", page.Status)
	require.NotEmpty(t, page.Error)
	require.Zero(t, page.HTTPStatus)
	require.Contains(t, page.Error, "Client.Timeout")
}

func TestAnalyzeInvalidInput(t *testing.T) {
	server := setupTestServer(t)

	ctx := t.Context()
	_, err := crawler.Analyze(ctx, crawler.Options{URL: server.URL})
	require.Error(t, err, "nil http client")

	_, err = crawler.Analyze(ctx, crawler.Options{URL: "ftp://x", HTTPClient: server.Client()})
	require.ErrorIs(t, err, crawler.ErrInvalidURL)
}

package crawler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"code/crawler"
)

func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	handler := http.HandlerFunc((func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/server-error":
			w.WriteHeader(http.StatusInternalServerError)
		case "/slow":
			time.Sleep(200 * time.Millisecond)
		default:
			http.NotFound(w, r)
		}
	}))

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server
}

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
			ctx := context.Background()
			opts := crawler.Options{
				URL:        server.URL + tt.path,
				HTTPClient: server.Client(),
			}
			data, err := crawler.Analyze(ctx, opts)
			require.NoError(t, err)

			var result crawler.Result
			errJSON := json.Unmarshal(data, &result)
			require.NoError(t, errJSON)

			require.Len(t, result.Pages, 1)
			page := result.Pages[0]
			require.Equal(t, tt.statusCode, page.HTTPStatus)
			require.Equal(t, tt.statusText, page.Status)
			require.Empty(t, page.Error)
			require.Equal(t, server.URL+tt.path, result.RootURL)
		})
	}
}

func TestAnalyzeNetworkError(t *testing.T) {
	server := setupTestServer(t)
	url := server.URL
	client := server.Client()
	server.Close()
	ctx := context.Background()

	data, err := crawler.Analyze(ctx, crawler.Options{URL: url, HTTPClient: client})
	require.NoError(t, err)

	var result crawler.Result
	errJSON := json.Unmarshal(data, &result)
	require.NoError(t, errJSON)

	require.Len(t, result.Pages, 1)
	page := result.Pages[0]
	require.Zero(t, page.HTTPStatus)
	require.Equal(t, "error", page.Status)
	require.NotEmpty(t, page.Error)
}

func TestAnalyzeTimeout(t *testing.T) {
	server := setupTestServer(t)
	client := server.Client()
	url := server.URL + "/slow"
	client.Timeout = 50 * time.Millisecond
	ctx := context.Background()

	data, err := crawler.Analyze(ctx, crawler.Options{URL: url, HTTPClient: client})
	require.NoError(t, err)
	var result crawler.Result
	errJSON := json.Unmarshal(data, &result)
	require.NoError(t, errJSON)

	require.Len(t, result.Pages, 1)
	page := result.Pages[0]
	require.Equal(t, "error", page.Status)
	require.NotEmpty(t, page.Error)
	require.Zero(t, page.HTTPStatus)
	require.Contains(t, page.Error, "Client.Timeout")
}

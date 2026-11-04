package crawler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"code/crawler"
)

func TestPageStatus(t *testing.T) {
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
				Depth:      1,
				HTTPClient: server.Client(),
			})

			require.Equal(t, tt.statusCode, page.HTTPStatus)
			require.Equal(t, tt.statusText, page.Status)
			require.Empty(t, page.Error)
		})
	}
}

func TestPageNetworkError(t *testing.T) {
	server := setupTestServer(t)
	url := server.URL
	client := server.Client()
	server.Close()

	page := firstPage(t, crawler.Options{URL: url, Depth: 1, HTTPClient: client})

	require.Zero(t, page.HTTPStatus)
	require.Equal(t, "error", page.Status)
	require.NotEmpty(t, page.Error)
	require.Nil(t, page.BrokenLinks)
	require.Nil(t, page.Assets)

	data, err := json.Marshal(page)
	require.NoError(t, err)

	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &got))
	require.JSONEq(t, "null", string(got["broken_links"]))
	require.JSONEq(t, "null", string(got["assets"]))
}

func TestPageTimeout(t *testing.T) {
	tests := []struct {
		name              string
		timeout           time.Duration
		clientTimeout     time.Duration
		wantErrorContains string
	}{
		{
			name:              "request timeout",
			timeout:           50 * time.Millisecond,
			wantErrorContains: context.DeadlineExceeded.Error(),
		},
		{
			name:              "shorter client timeout",
			timeout:           200 * time.Millisecond,
			clientTimeout:     50 * time.Millisecond,
			wantErrorContains: "Client.Timeout",
		},
		{
			name:              "zero timeout preserves client timeout",
			clientTimeout:     50 * time.Millisecond,
			wantErrorContains: "Client.Timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := setupTestServer(t)
			client := server.Client()
			client.Timeout = tt.clientTimeout

			page := firstPage(t, crawler.Options{
				URL:        server.URL + "/slow",
				Depth:      1,
				HTTPClient: client,
				Timeout:    tt.timeout,
			})

			require.Equal(t, "error", page.Status)
			require.Zero(t, page.HTTPStatus)
			require.Contains(t, page.Error, tt.wantErrorContains)
			require.Equal(t, tt.clientTimeout, client.Timeout)
		})
	}
}

func TestPageTimeoutWhileReadingBody(t *testing.T) {
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<title>Home</title>"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))

	page := firstPage(t, crawler.Options{
		URL:        server.URL,
		Depth:      1,
		HTTPClient: server.Client(),
		Timeout:    50 * time.Millisecond,
	})

	require.Equal(t, "error", page.Status)
	require.Equal(t, http.StatusOK, page.HTTPStatus)
	require.Contains(t, page.Error, context.DeadlineExceeded.Error())
}

func TestPageRetriesUseTimeoutPerAttempt(t *testing.T) {
	var requests atomic.Int32
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		time.Sleep(30 * time.Millisecond)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))

	page := firstPage(t, crawler.Options{
		URL:        server.URL,
		Depth:      1,
		HTTPClient: server.Client(),
		Timeout:    80 * time.Millisecond,
		Retries:    2,
	})

	require.Equal(t, "error", page.Status)
	require.Equal(t, http.StatusServiceUnavailable, page.HTTPStatus)
	require.Equal(t, int32(3), requests.Load())
}

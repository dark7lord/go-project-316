package crawler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	handler := http.HandlerFunc((func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("main route"))
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
		name    string
		path    string
		status  int
		wantErr bool
	}{
		{
			name:   "success",
			path:   "/",
			status: http.StatusOK,
		},
		{
			name:    "not found",
			path:    "/missing",
			status:  http.StatusNotFound,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			opts := Options{
				URL:        server.URL + tt.path,
				HTTPClient: server.Client(),
			}
			data, err := Analyze(ctx, opts)
			require.NoError(t, err)

			var result Result
			errJSON := json.Unmarshal(data, &result)
			require.NoError(t, errJSON)

			require.Len(t, result.Pages, 1)
			page := result.Pages[0]
			require.Equal(t, page.Status, tt.status)
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

	data, err := Analyze(ctx, Options{URL: url, HTTPClient: client})
	require.NoError(t, err)

	var result Result
	errJSON := json.Unmarshal(data, &result)
	require.NoError(t, errJSON)

	require.Len(t, result.Pages, 1)
	page := result.Pages[0]
	require.Zero(t, page.Status)
	require.NotEmpty(t, page.Error)
}

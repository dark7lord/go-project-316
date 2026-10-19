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

	files := http.FileServer(http.Dir("testdata"))

	handler := http.HandlerFunc((func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/server-error":
			w.WriteHeader(http.StatusInternalServerError)
		case "/slow":
			time.Sleep(200 * time.Millisecond)
		default:
			files.ServeHTTP(w, r)
		}
	}))

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server
}

// firstPage runs Analyze and returns the only page from the report.
func firstPage(t *testing.T, opts crawler.Options) crawler.Page {
	t.Helper()

	data, err := crawler.Analyze(context.Background(), opts)
	require.NoError(t, err)

	var report crawler.Report
	require.NoError(t, json.Unmarshal(data, &report))
	require.Len(t, report.Pages, 1)

	return report.Pages[0]
}

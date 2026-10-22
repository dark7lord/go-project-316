package crawler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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

	data, err := crawler.Analyze(t.Context(), opts)
	require.NoError(t, err)

	var report crawler.Report
	require.NoError(t, json.Unmarshal(data, &report))
	require.Len(t, report.Pages, 1)

	return report.Pages[0]
}

func pageDepths(t *testing.T, opts crawler.Options) map[string]uint {
	t.Helper()

	data, err := crawler.Analyze(t.Context(), opts)
	require.NoError(t, err)

	var report crawler.Report
	require.NoError(t, json.Unmarshal(data, &report))

	result := map[string]uint{}

	for _, page := range report.Pages {
		u, err := url.Parse(page.URL)
		require.NoError(t, err)
		require.NotContains(t, result, u.Path, "page appears twice in report")

		result[u.Path] = page.Depth
	}

	return result
}

package crawler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"code/crawler"
)

func newTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server
}

func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	files := http.FileServer(http.Dir("testdata"))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/server-error":
			w.WriteHeader(http.StatusInternalServerError)
		case "/slow":
			time.Sleep(200 * time.Millisecond)
		default:
			files.ServeHTTP(w, r)
		}
	})

	return newTestServer(t, handler)
}

func analyzeReport(t *testing.T, opts crawler.Options) crawler.Report {
	t.Helper()

	return analyzeReportWithContext(t.Context(), t, opts)
}

func analyzeReportWithContext(ctx context.Context, t *testing.T, opts crawler.Options) crawler.Report {
	t.Helper()

	data, err := crawler.Analyze(ctx, opts)
	require.NoError(t, err)

	var report crawler.Report
	require.NoError(t, json.Unmarshal(data, &report))

	return report
}

// firstPage runs Analyze and returns the root page from the report.
func firstPage(t *testing.T, opts crawler.Options) crawler.Page {
	t.Helper()

	report := analyzeReport(t, opts)
	require.NotEmpty(t, report.Pages)

	return report.Pages[0]
}

func pageDepthsFromReport(t *testing.T, report crawler.Report) map[string]uint {
	t.Helper()

	result := map[string]uint{}

	for _, page := range report.Pages {
		u, err := url.Parse(page.URL)
		require.NoError(t, err)
		require.NotContains(t, result, u.Path, "page appears twice in report")

		result[u.Path] = page.Depth
	}

	return result
}

func reportURLs(report crawler.Report) []string {
	urls := make([]string, len(report.Pages))
	for i, page := range report.Pages {
		urls[i] = page.URL
	}

	return urls
}

func newCancelServer(t *testing.T) (*httptest.Server, context.Context) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	files := http.FileServer(http.Dir("testdata"))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = fmt.Fprintf(w, `<a href="/cancel">x</a>`)
		case "/cancel":
			cancel()
			<-r.Context().Done()
		default:
			files.ServeHTTP(w, r)
		}
	})

	return newTestServer(t, handler), ctx
}

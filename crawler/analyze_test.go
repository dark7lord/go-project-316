package crawler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"code/crawler"
)

func TestAnalyzeInvalidInput(t *testing.T) {
	server := setupTestServer(t)

	ctx := t.Context()
	_, err := crawler.Analyze(ctx, crawler.Options{URL: server.URL})
	require.Error(t, err, "nil http client")

	_, err = crawler.Analyze(ctx, crawler.Options{URL: "ftp://x", HTTPClient: server.Client()})
	require.ErrorIs(t, err, crawler.ErrInvalidURL)
}

func TestAnalyzeCanceledContext(t *testing.T) {
	server := setupTestServer(t)

	canceledCtx, cancel := context.WithCancel(t.Context())
	cancel()

	opts := crawler.Options{URL: server.URL, HTTPClient: server.Client()}
	data, err := crawler.Analyze(canceledCtx, opts)
	require.NoError(t, err)

	var report crawler.Report
	require.NoError(t, json.Unmarshal(data, &report))
	require.Empty(t, report.Pages)
}

func TestAnalyzeJSONMatchesExampleWithIndenting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = fmt.Fprint(w, `
				<title>Example title</title>
				<meta name="description" content="Example description">
				<h1>Welcome</h1>
				<a href="/missing">missing</a>
				<img src="/static/logo.png">
			`)
		case "/missing":
			http.NotFound(w, r)
		case "/static/logo.png":
			_, _ = fmt.Fprint(w, "logo asset!!")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	options := crawler.Options{URL: server.URL, HTTPClient: server.Client()}
	unindentedJSON, err := crawler.Analyze(t.Context(), options)
	require.NoError(t, err)

	options.IndentJSON = 4
	indentedJSON, err := crawler.Analyze(t.Context(), options)
	require.NoError(t, err)
	require.NotEqual(t, unindentedJSON, indentedJSON)
	require.Contains(t, string(indentedJSON), "\n    \"root_url\"")

	var unindentedReport, indentedReport crawler.Report
	require.NoError(t, json.Unmarshal(unindentedJSON, &unindentedReport))
	require.NoError(t, json.Unmarshal(indentedJSON, &indentedReport))
	require.Len(t, unindentedReport.Pages, 1)
	require.Len(t, indentedReport.Pages, 1)
	require.NotZero(t, unindentedReport.GeneratedAt)
	require.NotZero(t, unindentedReport.Pages[0].DiscoveredAt)
	require.NotZero(t, indentedReport.GeneratedAt)
	require.NotZero(t, indentedReport.Pages[0].DiscoveredAt)

	wantJSON := func(report crawler.Report) string {
		return fmt.Sprintf(`{
			"root_url": %q,
			"depth": 0,
			"generated_at": %q,
			"pages": [{
				"url": %q,
				"depth": 0,
				"http_status": 200,
				"status": "ok",
				"error": "",
				"seo": {
					"has_title": true,
					"title": "Example title",
					"has_description": true,
					"description": "Example description",
					"has_h1": true
				},
				"broken_links": [{
					"url": %q,
					"status_code": 404,
					"error": "Not Found"
				}],
				"assets": [{
					"url": %q,
					"type": "image",
					"status_code": 200,
					"size_bytes": 12,
					"error": ""
				}],
				"discovered_at": %q
			}]
		}`,
			server.URL,
			report.GeneratedAt.Format(time.RFC3339Nano),
			server.URL,
			server.URL+"/missing",
			server.URL+"/static/logo.png",
			report.Pages[0].DiscoveredAt.Format(time.RFC3339Nano),
		)
	}
	require.JSONEq(t, wantJSON(unindentedReport), string(unindentedJSON))
	require.JSONEq(t, wantJSON(indentedReport), string(indentedJSON))

	unindentedReport.GeneratedAt = time.Time{}
	indentedReport.GeneratedAt = time.Time{}
	unindentedReport.Pages[0].DiscoveredAt = time.Time{}
	indentedReport.Pages[0].DiscoveredAt = time.Time{}
	require.Equal(t, unindentedReport, indentedReport)
}

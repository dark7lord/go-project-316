package crawler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"code/crawler"
)

func TestAnalyzeInvalidInput(t *testing.T) {
	server := setupTestServer(t)

	tests := []struct {
		name       string
		opts       crawler.Options
		wantErrIs  error
		wantErrMsg string
	}{
		{
			name:       "nil http client",
			opts:       crawler.Options{URL: server.URL, Depth: 1},
			wantErrMsg: "http client is required",
		},
		{
			name:      "invalid url",
			opts:      crawler.Options{URL: "ftp://x", Depth: 1, HTTPClient: server.Client()},
			wantErrIs: crawler.ErrInvalidURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := crawler.Analyze(t.Context(), tt.opts)
			require.Error(t, err)

			if tt.wantErrIs != nil {
				require.ErrorIs(t, err, tt.wantErrIs)
			}
			if tt.wantErrMsg != "" {
				require.ErrorContains(t, err, tt.wantErrMsg)
			}
		})
	}
}

func TestAnalyzeDepth(t *testing.T) {
	server := setupTestServer(t)

	tests := []struct {
		name  string
		depth uint
		want  map[string]uint
	}{
		{
			name:  "depth 0",
			depth: 0,
			want: map[string]uint{
				"/": 0,
			},
		},
		{
			name:  "depth 1",
			depth: 1,
			want: map[string]uint{
				"/": 0,
			},
		},
		{
			name:  "depth 2",
			depth: 2,
			want: map[string]uint{
				"/":              0,
				"/about.html":    1,
				"/contacts.html": 1,
				"/not-found":     1,
			},
		},
		{
			name:  "depth 3",
			depth: 3,
			want: map[string]uint{
				"/":              0,
				"/about.html":    1,
				"/contacts.html": 1,
				"/not-found":     1,
				"/team.html":     2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := analyzeReport(t, crawler.Options{
				URL:        server.URL + "/",
				HTTPClient: server.Client(),
				Depth:      tt.depth,
			})

			require.Equal(t, tt.depth, report.Depth)
			require.Equal(t, tt.want, pageDepthsFromReport(t, report))
		})
	}
}

func TestAnalyzeCancellation(t *testing.T) {
	t.Run("canceled before start", func(t *testing.T) {
		server := setupTestServer(t)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		report := analyzeReportWithContext(ctx, t, crawler.Options{
			URL:        server.URL,
			Depth:      1,
			HTTPClient: server.Client(),
		})
		require.Empty(t, report.Pages)
		require.ErrorIs(t, ctx.Err(), context.Canceled)
	})

	t.Run("during page fetch", func(t *testing.T) {
		server, ctx := newCancelServer(t)

		report := analyzeReportWithContext(ctx, t, crawler.Options{
			URL:        server.URL + "/cancel",
			Depth:      1,
			HTTPClient: server.Client(),
		})
		require.Empty(t, report.Pages)
		require.ErrorIs(t, ctx.Err(), context.Canceled)
	})

	t.Run("during link check", func(t *testing.T) {
		server, ctx := newCancelServer(t)

		report := analyzeReportWithContext(ctx, t, crawler.Options{
			URL:        server.URL,
			Depth:      1,
			HTTPClient: server.Client(),
		})
		require.NotEmpty(t, report.Pages)
		require.Equal(t, "ok", report.Pages[0].Status)
		require.Empty(t, report.Pages[0].BrokenLinks)
		require.ErrorIs(t, ctx.Err(), context.Canceled)
	})
}

func TestAnalyzeEmptyCollectionsAreArrays(t *testing.T) {
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `<p>no assets</p>`)
	}))

	report := analyzeReport(t, crawler.Options{
		URL:        server.URL,
		Depth:      1,
		HTTPClient: server.Client(),
	})
	require.Len(t, report.Pages, 1)
	require.NotNil(t, report.Pages[0].BrokenLinks)
	require.Empty(t, report.Pages[0].BrokenLinks)
	require.NotNil(t, report.Pages[0].Assets)
	require.Empty(t, report.Pages[0].Assets)
}

func TestAnalyzeJSONMatchesExampleWithIndenting(t *testing.T) {
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	options := crawler.Options{URL: server.URL, Depth: 1, HTTPClient: server.Client()}
	unindentedJSON, err := crawler.Analyze(t.Context(), options)
	require.NoError(t, err)

	options.IndentJSON = true
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
			"depth": 1,
			"generated_at": %q,
			"pages": [{
				"url": %q,
				"depth": 0,
				"http_status": 200,
				"status": "ok",
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
					"size_bytes": 12
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

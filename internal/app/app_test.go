package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"code/crawler"
	"code/internal/app"
)

func runApp(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer

	fullArgs := append([]string{"hexlet-go-crawler"}, args...)
	ctx := context.Background()
	code = app.Run(ctx, fullArgs, &out, &errOut)

	return code, out.String(), errOut.String()
}

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"no args shows help", nil, 0, "<url>", ""},
		{"empty url shows help", []string{""}, 0, "<url>", ""},
		{"version", []string{"version"}, 0, "1.0.0", ""},
		{"too many args", []string{"a.com", "b.com"}, 1, "", "expected 1 url"},
		{"invalid flag value", []string{"--depth", "abc", "a.com"}, 1, "", "invalid value"},
		{"zero depth", []string{"--depth", "0", "https://example.com"}, 1, "", "analyze error: depth must be greater than 0"},
		{"invalid url", []string{"ftp://a.com"}, 1, "", "invalid url"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runApp(tt.args...)

			require.Equal(t, tt.wantCode, code)
			require.Contains(t, stdout, tt.wantStdout)
			require.Contains(t, stderr, tt.wantStderr)
		})
	}
}

func TestRunPrintsReport(t *testing.T) {
	receivedUserAgent := make(chan string, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case receivedUserAgent <- r.UserAgent():
		default:
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<title>Home</title>"))
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	const userAgent = "json-report-test-agent"
	code, stdout, stderr := runApp(
		"--user-agent", userAgent,
		server.URL,
	)
	require.Equal(t, 0, code, stderr)
	require.Contains(t, stdout, "\n    \"root_url\"")
	require.Equal(t, userAgent, <-receivedUserAgent)
	require.True(t, strings.HasSuffix(stdout, "\n"))
	require.False(t, strings.HasSuffix(stdout, "\n\n"))

	var report crawler.Report
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSuffix(stdout, "\n")), &report))
	require.Equal(t, server.URL, report.RootURL)
	require.Len(t, report.Pages, 1)
	require.Equal(t, "Home", report.Pages[0].SEO.Title)

	libraryJSON, err := crawler.Analyze(t.Context(), crawler.Options{
		URL:         server.URL,
		Depth:       10,
		Retries:     1,
		Timeout:     15 * time.Second,
		UserAgent:   userAgent,
		Concurrency: 4,
		IndentJSON:  true,
		HTTPClient:  &http.Client{},
	})
	require.NoError(t, err)

	var libraryReport crawler.Report
	require.NoError(t, json.Unmarshal(libraryJSON, &libraryReport))
	report.GeneratedAt = time.Time{}
	libraryReport.GeneratedAt = time.Time{}
	report.Pages[0].DiscoveredAt = time.Time{}
	libraryReport.Pages[0].DiscoveredAt = time.Time{}
	require.Equal(t, libraryReport, report)
}

func TestRunDepthCountsLevelsIncludingRoot(t *testing.T) {
	tests := []struct {
		depth     string
		wantPaths []string
	}{
		{depth: "1", wantPaths: []string{"/"}},
		{depth: "2", wantPaths: []string{"/", "/child"}},
	}

	for _, tt := range tests {
		t.Run("depth "+tt.depth, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/" {
					_, _ = w.Write([]byte(`<a href="/child">child</a>`))
					return
				}

				_, _ = w.Write([]byte(`<a href="/grandchild">grandchild</a>`))
			}))
			t.Cleanup(server.Close)

			code, stdout, stderr := runApp("--depth", tt.depth, server.URL)
			require.Equal(t, 0, code, stderr)

			var report crawler.Report
			require.NoError(t, json.Unmarshal([]byte(stdout), &report))
			require.Equal(t, uint(len(tt.wantPaths)), uint(report.Depth))
			require.Len(t, report.Pages, len(tt.wantPaths))

			gotPaths := make([]string, len(report.Pages))
			for i, page := range report.Pages {
				gotPaths[i] = strings.TrimPrefix(page.URL, server.URL)
				if gotPaths[i] == "" {
					gotPaths[i] = "/"
				}
			}
			require.Equal(t, tt.wantPaths, gotPaths)
		})
	}
}

func TestIndentJSONCanBeDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<title>Home</title>"))
	}))
	t.Cleanup(server.Close)

	code, stdout, stderr := runApp("--indent-json=false", server.URL)
	require.Equal(t, 0, code, stderr)
	require.True(t, strings.HasSuffix(stdout, "\n"))
	compactJSON := strings.TrimSuffix(stdout, "\n")
	require.NotContains(t, compactJSON, "\n")
	require.True(t, json.Valid([]byte(compactJSON)))
}

func TestIndentJSONDefaultsToIndented(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<title>Home</title>"))
	}))
	t.Cleanup(server.Close)

	code, stdout, stderr := runApp(server.URL)
	require.Equal(t, 0, code, stderr)
	require.Contains(t, stdout, "\n    \"root_url\"")
}

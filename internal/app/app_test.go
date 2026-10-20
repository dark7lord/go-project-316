package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<title>Home</title>"))
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	code, stdout, stderr := runApp(server.URL)
	require.Equal(t, 0, code, stderr)

	var report crawler.Report
	require.NoError(t, json.Unmarshal([]byte(stdout), &report))
	require.Equal(t, server.URL, report.RootURL)
	require.Len(t, report.Pages, 1)
	require.Equal(t, "Home", report.Pages[0].SEO.Title)
}

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	require.True(t, strings.HasSuffix(stdout, "\n"))
	require.False(t, strings.HasSuffix(stdout, "\n\n"))
	require.Equal(t, userAgent, <-receivedUserAgent)

	var report crawler.Report
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSuffix(stdout, "\n")), &report))
	require.Equal(t, server.URL, report.RootURL)
	require.Len(t, report.Pages, 1)
}

func TestRunForwardsDepth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			_, _ = w.Write([]byte(`<a href="/child">child</a>`))
			return
		}

		_, _ = w.Write([]byte(`<a href="/grandchild">grandchild</a>`))
	}))
	t.Cleanup(server.Close)

	code, stdout, stderr := runApp("--depth", "2", server.URL)
	require.Equal(t, 0, code, stderr)

	var report crawler.Report
	require.NoError(t, json.Unmarshal([]byte(stdout), &report))
	require.Equal(t, uint(2), report.Depth)
	require.Len(t, report.Pages, 2)
	require.Equal(t, []string{server.URL, server.URL + "/child"}, []string{
		report.Pages[0].URL,
		report.Pages[1].URL,
	})
}

func TestRunIndentJSON(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantIndented bool
	}{
		{name: "default", wantIndented: true},
		{name: "disabled", args: []string{"--indent-json=false"}, wantIndented: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("<title>Home</title>"))
			}))
			t.Cleanup(server.Close)

			args := make([]string, 0, len(tt.args)+1)
			args = append(args, tt.args...)
			args = append(args, server.URL)
			code, stdout, stderr := runApp(args...)
			require.Equal(t, 0, code, stderr)
			require.True(t, strings.HasSuffix(stdout, "\n"))

			compactJSON := strings.TrimSuffix(stdout, "\n")
			if tt.wantIndented {
				require.Contains(t, compactJSON, "\n    \"root_url\"")
			} else {
				require.NotContains(t, compactJSON, "\n")
			}
			require.True(t, json.Valid([]byte(compactJSON)))
		})
	}
}

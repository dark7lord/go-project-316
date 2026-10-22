package crawler_test

import (
	"context"
	"encoding/json"
	"testing"

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

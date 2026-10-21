package crawler_test

import (
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

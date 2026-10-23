package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newTestFetcher(t *testing.T, delay time.Duration) (*fetcher, *url.URL) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("delay: " + delay.String()))
	})

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	ftr := newFetcher(server.Client(), delay)

	return &ftr, u
}

func TestFetcherFirstRequestNoWait(t *testing.T) {
	delay := 50 * time.Millisecond
	ftr, u := newTestFetcher(t, delay)
	start := time.Now()

	resp, err := ftr.get(t.Context(), u)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	require.Less(t, time.Since(start), delay)
}

func TestFetcherKeepsInterval(t *testing.T) {
	delay := 50 * time.Millisecond
	ftr, u := newTestFetcher(t, delay)
	intervals := 4
	minElapsed := time.Duration(intervals) * delay
	start := time.Now()

	for range intervals + 1 {
		resp, err := ftr.get(t.Context(), u)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
	}

	require.GreaterOrEqual(t, time.Since(start), minElapsed)
}

func TestFetcherCancelStopsWaiting(t *testing.T) {
	ftr, u := newTestFetcher(t, 5*time.Second)
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	resp, err := ftr.get(ctx, u)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	start := time.Now()
	resp, err = ftr.get(ctx, u)
	if resp != nil {
		_ = resp.Body.Close()
	}

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, resp)
	require.Less(t, time.Since(start), time.Second)
}

func TestFetcherNoDelayNoWait(t *testing.T) {
	ftr, u := newTestFetcher(t, 0)
	start := time.Now()

	for range 5 {
		resp, err := ftr.get(t.Context(), u)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
	}

	require.Less(t, time.Since(start), 50*time.Millisecond)
}

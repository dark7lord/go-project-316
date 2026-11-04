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

	ftr := newFetcher(server.Client(), delay, 0, "", 0)

	return &ftr, u
}

func TestFetcherRateLimit(t *testing.T) {
	const delay = 50 * time.Millisecond

	tests := []struct {
		name     string
		delay    time.Duration
		requests int
		wantMin  time.Duration
		wantMax  time.Duration
	}{
		{
			name:     "first request is not delayed",
			delay:    delay,
			requests: 1,
			wantMax:  delay,
		},
		{
			name:     "keeps configured interval",
			delay:    delay,
			requests: 5,
			wantMin:  4 * delay,
		},
		{
			name:     "enforces minimum interval by default",
			requests: 5,
			wantMin:  4 * minRetryDelay,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ftr, u := newTestFetcher(t, tt.delay)
			start := time.Now()

			for range tt.requests {
				resp, err := ftr.get(t.Context(), u)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())
			}

			elapsed := time.Since(start)
			if tt.wantMin > 0 {
				require.GreaterOrEqual(t, elapsed, tt.wantMin)
			}
			if tt.wantMax > 0 {
				require.Less(t, elapsed, tt.wantMax)
			}
		})
	}
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

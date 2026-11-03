package crawler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newStatusFetcher(t *testing.T, retries uint, statuses ...int) (*fetcher, *url.URL, *atomic.Int32) {
	t.Helper()
	require.NotEmpty(t, statuses)

	var requests atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		index := int(requests.Add(1)) - 1
		if index >= len(statuses) {
			index = len(statuses) - 1
		}
		w.WriteHeader(statuses[index])
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	ftr := newFetcher(server.Client(), 0, retries, "")

	return &ftr, u, &requests
}

func TestFetcherHTTPRetryBehavior(t *testing.T) {
	tests := []struct {
		name         string
		retries      uint
		statuses     []int
		wantStatus   int
		wantRequests int32
	}{
		{
			name:         "success makes one request",
			retries:      3,
			statuses:     []int{http.StatusOK},
			wantStatus:   http.StatusOK,
			wantRequests: 1,
		},
		{
			name:         "temporary status retries",
			retries:      1,
			statuses:     []int{http.StatusInternalServerError, http.StatusOK},
			wantStatus:   http.StatusOK,
			wantRequests: 2,
		},
		{
			name:         "success on final retry",
			retries:      2,
			statuses:     []int{http.StatusServiceUnavailable, http.StatusServiceUnavailable, http.StatusOK},
			wantStatus:   http.StatusOK,
			wantRequests: 3,
		},
		{
			name:         "zero retries returns first response",
			statuses:     []int{http.StatusServiceUnavailable, http.StatusOK},
			wantStatus:   http.StatusServiceUnavailable,
			wantRequests: 1,
		},
		{
			name:         "client error does not retry",
			retries:      3,
			statuses:     []int{http.StatusNotFound, http.StatusOK},
			wantStatus:   http.StatusNotFound,
			wantRequests: 1,
		},
		{
			name:         "retry limit returns last response",
			retries:      2,
			statuses:     []int{http.StatusServiceUnavailable},
			wantStatus:   http.StatusServiceUnavailable,
			wantRequests: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ftr, u, requests := newStatusFetcher(t, tt.retries, tt.statuses...)

			resp, err := ftr.get(t.Context(), u)
			require.NoError(t, err)
			require.Equal(t, tt.wantStatus, resp.StatusCode)
			require.NoError(t, resp.Body.Close())
			require.Equal(t, tt.wantRequests, requests.Load())
		})
	}
}

func TestFetcherRetriesTransportError(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack connection: %v", err)
				return
			}
			_ = conn.Close()

			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	ftr := newFetcher(server.Client(), 0, 1, "")

	resp, err := ftr.get(t.Context(), u)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, int32(2), requests.Load())
}

func TestFetcherReturnsLastTransportError(t *testing.T) {
	var requests atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack connection: %v", err)
			return
		}
		_ = conn.Close()
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	ftr := newFetcher(server.Client(), 0, 2, "")

	resp, err := ftr.get(t.Context(), u)
	if resp != nil {
		_ = resp.Body.Close()
	}

	require.Nil(t, resp)
	require.Error(t, err)
	require.Equal(t, int32(3), requests.Load())
}

func TestFetcherCancelStopsWaitingBeforeRetry(t *testing.T) {
	var requests atomic.Int32
	firstRequest := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		close(firstRequest)
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	ftr := newFetcher(server.Client(), 5*time.Second, 1, "")

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	respCh := make(chan *http.Response, 1)
	errCh := make(chan error, 1)
	go func() {
		resp, err := ftr.get(ctx, u)
		if resp != nil {
			_ = resp.Body.Close()
		}

		respCh <- resp
		errCh <- err
	}()

	<-firstRequest
	resp := <-respCh
	err = <-errCh
	if resp != nil {
		_ = resp.Body.Close()
	}

	require.Nil(t, resp)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, int32(1), requests.Load())
}

func TestFetcherDoesNotRetryCanceledContext(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	t.Cleanup(server.Close)

	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	ftr := newFetcher(server.Client(), 0, 3, "")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	resp, err := ftr.get(ctx, u)
	if resp != nil {
		_ = resp.Body.Close()
	}

	require.Nil(t, resp)
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, requests.Load())
}

type responseSequence struct {
	responses []*http.Response
}

func (s *responseSequence) RoundTrip(*http.Request) (*http.Response, error) {
	resp := s.responses[0]
	s.responses = s.responses[1:]

	return resp, nil
}

type closeTrackingBody struct {
	closed bool
}

func (*closeTrackingBody) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (b *closeTrackingBody) Close() error {
	b.closed = true

	return nil
}

func TestFetcherClosesIntermediateResponseBody(t *testing.T) {
	intermediate := &closeTrackingBody{}
	final := &closeTrackingBody{}
	client := &http.Client{Transport: &responseSequence{responses: []*http.Response{
		{StatusCode: http.StatusInternalServerError, Header: make(http.Header), Body: intermediate},
		{StatusCode: http.StatusOK, Header: make(http.Header), Body: final},
	}}}
	u, err := url.Parse("http://example.test/")
	require.NoError(t, err)
	ftr := newFetcher(client, 0, 1, "")

	resp, err := ftr.get(t.Context(), u)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.True(t, intermediate.closed)
	require.False(t, final.closed)

	require.NoError(t, resp.Body.Close())
	require.True(t, final.closed)
}

func TestFetcherAppliesDelayBetweenRetries(t *testing.T) {
	const delay = 150 * time.Millisecond
	var requests atomic.Int32
	starts := make(chan time.Time, 2)
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		starts <- time.Now()
		if requests.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	ftr := newFetcher(server.Client(), delay, 1, "")

	resp, err := ftr.get(t.Context(), u)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	firstStart := <-starts
	retryStart := <-starts
	require.GreaterOrEqual(t, retryStart.Sub(firstStart), delay-10*time.Millisecond)
}

func TestFetcherWaitsBeforeRetryByDefault(t *testing.T) {
	var requests atomic.Int32
	starts := make(chan time.Time, 2)
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		starts <- time.Now()
		if requests.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	ftr := newFetcher(server.Client(), 0, 1, "")

	resp, err := ftr.get(t.Context(), u)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	firstStart := <-starts
	retryStart := <-starts
	require.GreaterOrEqual(t, retryStart.Sub(firstStart), 90*time.Millisecond)
}

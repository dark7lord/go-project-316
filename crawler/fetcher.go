package crawler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const minRetryDelay = 100 * time.Millisecond

type fetcher struct {
	client    *http.Client
	delay     time.Duration
	retries   uint
	userAgent string
	timeout   time.Duration
	limiter   *requestLimiter
}

type requestLimiter struct {
	mu   sync.Mutex
	next time.Time
}

func newFetcher(
	client *http.Client,
	delay time.Duration,
	retries uint,
	userAgent string,
	timeout time.Duration,
) fetcher {
	return fetcher{
		client:    client,
		delay:     delay,
		retries:   retries,
		userAgent: userAgent,
		timeout:   timeout,
		limiter:   &requestLimiter{},
	}
}

type cancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (body *cancelBody) Read(p []byte) (int, error) {
	n, err := body.ReadCloser.Read(p)
	if err != nil {
		body.cancel()
	}

	return n, err
}

func (body *cancelBody) Close() error {
	body.cancel()
	return body.ReadCloser.Close()
}

func isRetryable(res *http.Response, err error) bool {
	if err != nil {
		return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
	}
	code := res.StatusCode

	return (code >= 500 && code <= 599) ||
		code == http.StatusRequestTimeout ||
		code == http.StatusTooManyRequests
}

func (ftr *fetcher) wait(ctx context.Context) error {
	limiter := ftr.limiter
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	now := time.Now()
	if !limiter.next.IsZero() && now.Before(limiter.next) {
		timer := time.NewTimer(time.Until(limiter.next))
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}

	limiter.next = time.Now().Add(max(minRetryDelay, ftr.delay))

	return nil
}

func (ftr *fetcher) get(ctx context.Context, u *url.URL) (*http.Response, error) {
	remainingRetries := ftr.retries
	for {
		if err := ftr.wait(ctx); err != nil {
			return nil, err
		}

		res, err := ftr.fetchOnce(ctx, u)

		if !isRetryable(res, err) || remainingRetries == 0 {
			return res, err
		}

		if res != nil {
			_ = res.Body.Close()
		}
		remainingRetries--
	}
}

func (ftr *fetcher) fetchOnce(ctx context.Context, u *url.URL) (*http.Response, error) {
	attemptCtx, cancel := ctx, func() {}
	if ftr.timeout > 0 {
		attemptCtx, cancel = context.WithTimeout(ctx, ftr.timeout)
	}

	req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		cancel()
		return nil, err
	}
	if ftr.userAgent != "" {
		req.Header.Set("User-Agent", ftr.userAgent)
	}

	res, err := ftr.client.Do(req)
	if err != nil {
		cancel()
		return res, err
	}

	res.Body = &cancelBody{ReadCloser: res.Body, cancel: cancel}

	return res, nil
}

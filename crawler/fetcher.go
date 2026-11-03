package crawler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const minRetryDelay = 100 * time.Millisecond

type fetcher struct {
	client  *http.Client
	delay   time.Duration
	retries uint
	limiter *requestLimiter
}

type requestLimiter struct {
	mu   sync.Mutex
	next time.Time
}

func newFetcher(client *http.Client, delay time.Duration, retries uint) fetcher {
	return fetcher{client, delay, retries, &requestLimiter{}}
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
	if err := ftr.wait(ctx); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	res, err := ftr.client.Do(req)
	for range ftr.retries {
		if !isRetryable(res, err) {
			break
		}

		if res != nil {
			_ = res.Body.Close()
		}

		if err := ftr.wait(ctx); err != nil {
			return nil, err
		}

		res, err = ftr.client.Do(req)
	}

	return res, err
}

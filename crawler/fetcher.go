package crawler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"
)

const minRetryDelay = 100 * time.Millisecond

type fetcher struct {
	client  *http.Client
	delay   time.Duration
	timer   *time.Timer
	retries uint
}

func newFetcher(client *http.Client, delay time.Duration, retries uint) fetcher {
	var timer *time.Timer
	if delay > 0 {
		timer = time.NewTimer(0)
	}

	return fetcher{client, delay, timer, retries}
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
	if ftr.timer == nil {
		if err := ctx.Err(); err != nil {
			return err
		}

		ftr.timer = time.NewTimer(minRetryDelay)

		return nil
	}

	select {
	case <-ctx.Done():
		ftr.timer.Stop()
		return ctx.Err()
	case <-ftr.timer.C:
		ftr.timer.Reset(max(minRetryDelay, ftr.delay))
	}

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

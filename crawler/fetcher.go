package crawler

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

type fetcher struct {
	client *http.Client
	delay  time.Duration
	timer  *time.Timer
}

func newFetcher(client *http.Client, delay time.Duration) fetcher {
	var timer *time.Timer
	if delay > 0 {
		timer = time.NewTimer(0)
	}

	return fetcher{client, delay, timer}
}

func (ftr *fetcher) get(ctx context.Context, u *url.URL) (*http.Response, error) {
	if ftr.timer != nil {
		select {
		case <-ctx.Done():
			ftr.timer.Stop()
			return nil, ctx.Err()
		case <-ftr.timer.C:
			ftr.timer.Reset(ftr.delay)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	return ftr.client.Do(req)
}

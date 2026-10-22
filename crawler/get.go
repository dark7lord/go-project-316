package crawler

import (
	"context"
	"net/http"
	"net/url"
)

func (crw *crawler) get(ctx context.Context, u *url.URL) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	return crw.client.Do(req)
}

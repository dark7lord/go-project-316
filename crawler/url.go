package crawler

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrInvalidURL is returned when the url can't be crawled.
var ErrInvalidURL = errors.New("invalid url")

// normalizeURL adds http:// if needed and validates an http(s) url.
func normalizeURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("%w: %q", ErrInvalidURL, raw)
	}

	return u, nil
}

// resolveURL makes href absolute against base, false means skip it.
func resolveURL(base *url.URL, href string) (*url.URL, bool) {
	trimmed := strings.TrimSpace(href)
	if trimmed == "" {
		return nil, false
	}

	if strings.HasPrefix(trimmed, "#") {
		return nil, false
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, false
	}

	u := base.ResolveReference(parsed)

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, false
	}

	return u, true
}

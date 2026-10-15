package crawler

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrInvalidURL is returned when the url can't be crawled.
var ErrInvalidURL = errors.New("invalid url")

// normalizeURL trims the url, adds http:// when the scheme is missing
// and checks that it is an http(s) url with a host.
func normalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("%w: %q", ErrInvalidURL, raw)
	}

	return u.String(), nil
}

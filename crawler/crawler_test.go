package crawler_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"code/crawler"
)

func TestCrawlDepth(t *testing.T) {
	t.Skip("crawl not implemented")
	server := setupTestServer(t)

	tests := []struct {
		name  string
		depth uint
		want  map[string]uint
	}{
		{
			name:  "depth 0",
			depth: 0,
			want: map[string]uint{
				"/": 0,
			},
		},
		{
			name:  "depth 1",
			depth: 1,
			want: map[string]uint{
				"/":              0,
				"/about.html":    1,
				"/contacts.html": 1,
				"/not-found":     1,
			},
		},
		{
			name:  "depth 2",
			depth: 2,
			want: map[string]uint{
				"/":              0,
				"/about.html":    1,
				"/contacts.html": 1,
				"/not-found":     1,
				"/team.html":     2,
			},
		},
		{
			name:  "depth 3",
			depth: 3,
			want: map[string]uint{
				"/":              0,
				"/about.html":    1,
				"/contacts.html": 1,
				"/not-found":     1,
				"/team.html":     2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := crawler.Options{
				URL:        server.URL + "/",
				HTTPClient: server.Client(),
				Depth:      tt.depth,
			}

			got := pageDepths(t, opts)
			require.Equal(t, tt.want, got)
		})
	}
}

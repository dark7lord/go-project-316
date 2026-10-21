package crawler

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newTestCrawler(t *testing.T) crawler {
	t.Helper()

	root, err := url.Parse("http://simple.test/blog")
	require.NoError(t, err)

	return crawler{
		rootURL: root,
		depth:   3,
		pages: []Page{{
			URL:        "http://simple.test/blog",
			HTTPStatus: 200,
			Status:     statusOK,
		}},
	}
}

func TestReport(t *testing.T) {
	crw := newTestCrawler(t)

	data, err := report(crw, 2)
	require.NoError(t, err)

	var got Report
	require.NoError(t, json.Unmarshal(data, &got))

	require.Equal(t, "http://simple.test/blog", got.RootURL)
	require.Equal(t, uint(3), got.Depth)
	require.WithinDuration(t, time.Now(), got.GeneratedAt, 2*time.Second)
	require.Equal(t, crw.pages, got.Pages)
}

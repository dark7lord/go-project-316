package crawler

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newTestCrawler(t *testing.T) *crawler {
	t.Helper()

	root, err := url.Parse("http://simple.test/blog")
	require.NoError(t, err)

	return &crawler{
		rootURL: root,
		depth:   3,
		pages: []Page{{
			URL:         "http://simple.test/blog",
			HTTPStatus:  200,
			Status:      statusOK,
			BrokenLinks: []BrokenLink{},
			Assets:      []Asset{},
		}},
	}
}

func TestReport(t *testing.T) {
	crw := newTestCrawler(t)

	data, err := crw.report()
	require.NoError(t, err)

	var got Report
	require.NoError(t, json.Unmarshal(data, &got))

	require.Equal(t, "http://simple.test/blog", got.RootURL)
	require.Equal(t, uint(3), got.Depth)
	require.WithinDuration(t, time.Now(), got.GeneratedAt, 2*time.Second)
	require.Equal(t, crw.pages, got.Pages)
}

func TestReportJSONContracts(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{
			name:  "asset omits empty error",
			value: Asset{},
			want:  `{"url":"","type":"","status_code":0,"size_bytes":0}`,
		},
		{
			name:  "broken link includes all fields",
			value: BrokenLink{},
			want:  `{"url":"","status_code":0,"error":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.value)
			require.NoError(t, err)
			require.JSONEq(t, tt.want, string(data))
		})
	}
}

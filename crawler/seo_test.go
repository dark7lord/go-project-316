package crawler

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

func TestExtractSEO(t *testing.T) {
	tests := []struct {
		name string
		file string
		want SEOResult
	}{
		{
			name: "good seo",
			file: "testdata/good-seo.html",
			want: SEOResult{
				HasTitle:       true,
				Title:          "Good SEO",
				HasDescription: true,
				Description:    "Page with SEO params",
				HasH1:          true,
			},
		},
		{
			name: "bad seo",
			file: "testdata/bad-seo.html",
			want: SEOResult{},
		},
		{
			name: "empty title",
			file: "testdata/empty-title.html",
			want: SEOResult{HasH1: true},
		},
		{
			name: "spaces are trimmed",
			file: "testdata/spaced-seo.html",
			want: SEOResult{
				HasTitle:       true,
				Title:          "Spaced title",
				HasDescription: true,
				Description:    "Spaced description",
			},
		},
		{
			name: "whitespace only is empty",
			file: "testdata/blank-seo.html",
			want: SEOResult{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open(tt.file)
			require.NoError(t, err)
			t.Cleanup(func() { _ = f.Close() })

			doc, err := html.Parse(f)
			require.NoError(t, err)

			require.Equal(t, tt.want, extractSEO(doc))
		})
	}
}

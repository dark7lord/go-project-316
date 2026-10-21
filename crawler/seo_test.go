package crawler

import (
	"os"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/require"
)

func TestExtractSEO(t *testing.T) {
	tests := []struct {
		name string
		file string
		want SEOResult
	}{
		{
			name: "good seo",
			file: "testdata/seo/good.html",
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
			file: "testdata/seo/bad.html",
			want: SEOResult{},
		},
		{
			name: "empty tags",
			file: "testdata/seo/empty-tags.html",
			want: SEOResult{
				HasTitle:       true,
				HasDescription: true,
				HasH1:          true,
			},
		},
		{
			name: "spaces are trimmed",
			file: "testdata/seo/spaced.html",
			want: SEOResult{
				HasTitle:       true,
				Title:          "Spaced title",
				HasDescription: true,
				Description:    "Spaced description",
			},
		},
		{
			name: "decoding html entities",
			file: "testdata/seo/entities.html",
			want: SEOResult{
				HasTitle:       true,
				Title:          "Tom & Jerry",
				HasDescription: true,
				Description:    "Use <h1> once",
			},
		},
		{
			name: "svg title is not page title",
			file: "testdata/seo/svg-title.html",
			want: SEOResult{HasH1: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open(tt.file)
			require.NoError(t, err)
			t.Cleanup(func() { _ = f.Close() })

			doc, err := goquery.NewDocumentFromReader(f)
			require.NoError(t, err)

			require.Equal(t, tt.want, extractSEO(doc))
		})
	}
}

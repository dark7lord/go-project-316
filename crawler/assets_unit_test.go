package crawler

import (
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/require"
)

func TestExtractAssets(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`
		<html>
			<head>
				<link rel="alternate stylesheet" href="/css/site.css">
				<link rel="icon" href="/favicon.ico">
				<script src="https://cdn.example.test/app.js"></script>
			</head>
			<body>
				<img src="../images/logo.png">
				<img src="../images/logo.png">
				<script src="data:text/javascript,alert(1)"></script>
			</body>
		</html>
	`))
	require.NoError(t, err)

	pageURL, err := url.Parse("https://example.test/docs/index.html")
	require.NoError(t, err)

	got := extractAssets(doc, pageURL)
	gotAssets := make([]Asset, len(got))
	for i, asset := range got {
		gotAssets[i] = Asset{URL: asset.URL.String(), Type: asset.Type}
	}

	require.Equal(t, []Asset{
		{URL: "https://example.test/images/logo.png", Type: assetTypeImage},
		{URL: "https://cdn.example.test/app.js", Type: assetTypeScript},
		{URL: "https://example.test/css/site.css", Type: assetTypeStyle},
	}, gotAssets)
}

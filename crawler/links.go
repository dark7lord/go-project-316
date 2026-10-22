package crawler

import (
	"context"
	"net/url"

	"github.com/PuerkitoBio/goquery"
)

type foundURL struct {
	URL   *url.URL
	depth uint
}

func (crw *crawler) checkLink(ctx context.Context, absURL *url.URL) (BrokenLink, bool) {
	u := absURL.String()
	result := BrokenLink{URL: u}

	resp, err := crw.get(ctx, absURL)
	if err != nil && ctx.Err() != nil {
		return result, false
	}
	if err != nil {
		result.Error = err.Error()
		return result, true
	}
	_ = resp.Body.Close()

	if code := resp.StatusCode; code >= 400 {
		result.StatusCode = code
		return result, true
	}

	return result, false
}

func (crw *crawler) checkLinks(
	ctx context.Context,
	hrefs []string,
	pageURL *url.URL,
	depth uint,
) ([]BrokenLink, []foundURL) {
	var (
		brokenLinks []BrokenLink
		foundLinks  []foundURL
	)

	checked := map[string]bool{}

	for _, href := range hrefs {
		absURL, ok := resolveURL(pageURL, href)
		if !ok {
			continue
		}

		key := absURL.String()
		if checked[key] {
			continue
		}

		checked[key] = true

		if link, broken := crw.checkLink(ctx, absURL); broken {
			brokenLinks = append(brokenLinks, link)
		}

		found := foundURL{URL: absURL, depth: depth + 1}
		foundLinks = append(foundLinks, found)
	}

	return brokenLinks, foundLinks
}

func extractLinks(doc *goquery.Document) []string {
	return doc.Find("a[href]").Map(func(_ int, s *goquery.Selection) string {
		return s.AttrOr("href", "")
	})
}

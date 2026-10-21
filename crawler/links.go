package crawler

import (
	"net/url"

	"github.com/PuerkitoBio/goquery"
)

func (crw *crawler) checkLink(absURL *url.URL) (Link, bool) {
	resp, err := crw.client.Get(absURL.String())
	if err != nil {
		return Link{URL: absURL.String(), Error: err.Error()}, true
	}
	_ = resp.Body.Close()

	if code := resp.StatusCode; code >= 400 {
		return Link{URL: absURL.String(), StatusCode: code}, true
	}

	return Link{}, false
}

func (crw *crawler) checkLinks(hrefs []string, pageURL *url.URL) []Link {
	var brokenLinks []Link

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

		if link, broken := crw.checkLink(absURL); broken {
			brokenLinks = append(brokenLinks, link)
		}
	}

	return brokenLinks
}

func extractLinks(doc *goquery.Document) []string {
	return doc.Find("a[href]").Map(func(_ int, s *goquery.Selection) string {
		return s.AttrOr("href", "")
	})
}

package crawler

import (
	"fmt"
	"net/url"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// setError marks the page as failed with err.
func (p *Page) setError(err error) {
	p.Status = statusError
	p.Error = err.Error()
}

func (crw *crawler) fetchPage(pageURL *url.URL, depth uint) (page Page, liveLinks []foundURL) {
	strURL := pageURL.String()
	page = Page{
		URL:          strURL,
		Depth:        depth,
		DiscoveredAt: time.Now().UTC(),
	}

	defer func() {
		if r := recover(); r != nil {
			page.setError(fmt.Errorf("panic: %v", r))
		}
	}()

	resp, err := crw.client.Get(strURL)
	if err != nil {
		page.setError(err)
		return page, liveLinks
	}
	defer func() { _ = resp.Body.Close() }()

	page.HTTPStatus = resp.StatusCode
	page.Status = statusOK
	if resp.StatusCode >= 400 {
		page.Status = statusError
		return page, liveLinks
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		page.setError(err)
		return page, liveLinks
	}
	links := extractLinks(doc)
	page.SEO = extractSEO(doc)
	reqURL := resp.Request.URL
	brokenLinks, liveLinks := crw.checkLinks(links, reqURL, depth)
	page.BrokenLinks = brokenLinks

	return page, liveLinks
}

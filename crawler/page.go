package crawler

import (
	"fmt"
	"net/url"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func (crw *crawler) fetchPage(pageURL *url.URL) (page Page) {
	strURL := pageURL.String()
	page = Page{
		URL:          strURL,
		DiscoveredAt: time.Now().UTC(),
	}

	defer func() {
		if r := recover(); r != nil {
			page.Status = statusError
			page.Error = fmt.Sprint("panic: ", r)
		}
	}()

	crw.visited[strURL] = true
	resp, err := crw.client.Get(strURL)
	if err != nil {
		page.Status = statusError
		page.Error = err.Error()
		return page
	}
	defer func() { _ = resp.Body.Close() }()

	page.HTTPStatus = resp.StatusCode
	page.Status = statusOK
	if resp.StatusCode >= 400 {
		page.Status = statusError
		return page
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		page.Error = err.Error()
		return page
	}
	links := extractLinks(doc)
	page.SEO = extractSEO(doc)
	reqURL := resp.Request.URL
	page.BrokenLinks = crw.checkLinks(links, reqURL)

	return page
}

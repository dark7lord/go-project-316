package crawler

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type crawler struct {
	client  *http.Client
	rootURL *url.URL
	depth   uint
	visited map[string]bool
	pages   []Page
}

const (
	statusOK    = "ok"
	statusError = "error"
)

func extractLinks(doc *goquery.Document) []string {
	return doc.Find("a[href]").Map(func(_ int, s *goquery.Selection) string {
		return s.AttrOr("href", "")
	})
}

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

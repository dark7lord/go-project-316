package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	statusOK    = "ok"
	statusError = "error"
)

// Page is a single crawled page in the report.
type Page struct {
	URL          string       `json:"url"`
	Depth        uint         `json:"depth"`
	HTTPStatus   int          `json:"http_status"`
	Status       string       `json:"status"`
	Error        string       `json:"error"`
	BrokenLinks  []BrokenLink `json:"broken_links,omitempty"`
	DiscoveredAt time.Time    `json:"discovered_at"`
	SEO          SEOResult    `json:"seo"`
}

func newPage(u *url.URL, depth uint) Page {
	return Page{
		URL:          u.String(),
		Depth:        depth,
		DiscoveredAt: time.Now().UTC(),
	}
}

func (p *Page) setError(err error) {
	p.Status = statusError
	p.Error = err.Error()
}

func (p *Page) setHTTPStatus(code int) {
	p.HTTPStatus = code

	status := statusOK
	if code >= 400 {
		status = statusError
	}

	p.Status = status
}

func (crw *crawler) parsePage(ctx context.Context, page *Page, resp *http.Response) []foundURL {
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		page.setError(err)
		return nil
	}

	page.SEO = extractSEO(doc)
	links := extractLinks(doc)
	reqURL := resp.Request.URL
	brokenLinks, liveLinks := crw.checkLinks(ctx, links, reqURL, page.Depth)
	page.BrokenLinks = brokenLinks

	return liveLinks
}

func (crw *crawler) fetchPage(
	ctx context.Context,
	pageURL *url.URL,
	depth uint,
) (page Page, liveLinks []foundURL, ok bool) {
	page = newPage(pageURL, depth)

	defer func() {
		if r := recover(); r != nil {
			page.setError(fmt.Errorf("panic: %v", r))
			ok = true
		}
	}()

	resp, err := crw.get(ctx, pageURL)
	if err != nil && ctx.Err() != nil {
		return page, liveLinks, false
	}
	if err != nil {
		page.setError(err)
		return page, liveLinks, true
	}
	defer func() { _ = resp.Body.Close() }()

	page.setHTTPStatus(resp.StatusCode)

	if page.Status == statusError {
		return page, liveLinks, true
	}

	liveLinks = crw.parsePage(ctx, &page, resp)

	return page, liveLinks, true
}

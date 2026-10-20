package crawler

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type crawler struct {
	client  *http.Client
	rootURL url.URL
	visited map[string]bool
	pages   []Page
}

const (
	statusOK    = "ok"
	statusError = "error"
)

func extractLink(n *html.Node) (string, bool) {
	if n.Type != html.ElementNode || n.DataAtom != atom.A {
		return "", false
	}

	index := slices.IndexFunc(n.Attr, func(attr html.Attribute) bool {
		return attr.Key == "href"
	})

	if index < 0 {
		return "", false
	}

	return n.Attr[index].Val, true
}

func extractLinks(doc *html.Node) []string {
	var links []string

	for node := range doc.Descendants() {
		link, ok := extractLink(node)
		if ok {
			links = append(links, link)
		}
	}

	return links
}

func (crw *crawler) checkLink(absURL string) (Link, bool) {
	resp, err := crw.client.Get(absURL)
	if err != nil {
		return Link{URL: absURL, Error: err.Error()}, true
	}
	_ = resp.Body.Close()

	if code := resp.StatusCode; code >= 400 {
		return Link{URL: absURL, StatusCode: code}, true
	}

	return Link{}, false
}

func (crw *crawler) checkLinks(links []string, pageURL *url.URL) []Link {
	var brokenLinks []Link

	checked := map[string]bool{}

	for _, link := range links {
		absURL, ok := resolveLink(pageURL, link)
		if !ok || checked[absURL] {
			continue
		}
		checked[absURL] = true

		if href, broken := crw.checkLink(absURL); broken {
			brokenLinks = append(brokenLinks, href)
		}
	}

	return brokenLinks
}

func (crw *crawler) fetchPage(url string) (page Page) {
	page = Page{
		URL:          url,
		DiscoveredAt: time.Now().UTC(),
	}

	defer func() {
		if r := recover(); r != nil {
			page.Status = statusError
			page.Error = fmt.Sprint("panic: ", r)
		}
	}()

	crw.visited[url] = true
	resp, err := crw.client.Get(url)
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

	doc, err := html.Parse(resp.Body)
	if err != nil {
		page.Error = err.Error()
		return page
	}
	links := extractLinks(doc)
	page.SEO = extractSEO(doc)

	pageURL := resp.Request.URL
	page.BrokenLinks = crw.checkLinks(links, pageURL)

	return page
}

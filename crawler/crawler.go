package crawler

import (
	"errors"
	"net/http"
	"net/url"
)

type crawler struct {
	client     *http.Client
	rootURL    *url.URL
	host       string
	depth      uint
	visited    map[string]bool
	pages      []Page
	indentJSON uint
}

func newCrawler(opts Options) (*crawler, error) {
	if opts.HTTPClient == nil {
		return nil, errors.New("http client is required")
	}

	normURL, err := normalizeURL(opts.URL)
	if err != nil {
		return nil, err
	}

	return &crawler{
		rootURL:    normURL,
		host:       normURL.Host,
		client:     opts.HTTPClient,
		depth:      opts.Depth,
		indentJSON: opts.IndentJSON,
		visited:    map[string]bool{},
	}, nil
}

func (crw *crawler) crawl() {
	queue := []foundURL{{URL: crw.rootURL, depth: 0}}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		strURL := curr.URL.JoinPath().String()
		if curr.URL.Host != crw.host || crw.visited[strURL] || curr.depth > crw.depth {
			continue
		}

		crw.visited[strURL] = true
		page, nextLinks := crw.fetchPage(curr.URL, curr.depth)
		crw.pages = append(crw.pages, page)

		for _, nlk := range nextLinks {
			key := nlk.URL.JoinPath().String()
			if _, hasVisited := crw.visited[key]; hasVisited {
				continue
			}

			queue = append(queue, nlk)
		}
	}
}

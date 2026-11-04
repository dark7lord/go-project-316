package crawler

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"sync"
)

type crawler struct {
	fetcher      fetcher
	rootURL      *url.URL
	host         string
	depth        uint
	concurrency  uint
	visited      map[string]bool
	pages        []Page
	indentJSON   bool
	assetCacheMu sync.Mutex
	assetCache   map[string]*assetCacheEntry
}

type crawlResult struct {
	page      Page
	nextLinks []foundURL
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
		fetcher: newFetcher(
			opts.HTTPClient,
			opts.Delay,
			opts.Retries,
			opts.UserAgent,
			opts.Timeout,
		),
		rootURL:     normURL,
		host:        normURL.Host,
		depth:       opts.Depth,
		concurrency: max(uint(1), opts.Concurrency),
		indentJSON:  opts.IndentJSON,
		visited:     map[string]bool{},
		pages:       []Page{},
		assetCache:  map[string]*assetCacheEntry{},
	}, nil
}

func (crw *crawler) crawl(ctx context.Context) {
	queue := []foundURL{{URL: crw.rootURL, depth: 0}}

	for len(queue) > 0 {
		if ctx.Err() != nil {
			break
		}

		currentLevel := crw.nextLevel(queue)
		if len(currentLevel) == 0 {
			break
		}

		results := crw.processLevel(ctx, currentLevel)
		queue = nil
		for _, result := range results {
			crw.pages = append(crw.pages, result.page)
			queue = append(queue, result.nextLinks...)
		}
	}

	sort.Slice(crw.pages, func(a, b int) bool {
		if crw.pages[a].Depth != crw.pages[b].Depth {
			return crw.pages[a].Depth < crw.pages[b].Depth
		}

		return crw.pages[a].URL < crw.pages[b].URL
	})
}

func (crw *crawler) nextLevel(queue []foundURL) []foundURL {
	level := make([]foundURL, 0, len(queue))
	for _, candidate := range queue {
		key := candidate.URL.JoinPath().String()
		if candidate.URL.Host != crw.host || crw.visited[key] ||
			candidate.depth > 0 && candidate.depth >= crw.depth {
			continue
		}

		crw.visited[key] = true
		level = append(level, candidate)
	}

	return level
}

func (crw *crawler) processLevel(ctx context.Context, level []foundURL) []crawlResult {
	linksCh := make(chan foundURL, len(level))
	results := make(chan crawlResult, len(level))
	workerCount := min(int(crw.concurrency), len(level))

	var wg sync.WaitGroup
	for range workerCount {
		wg.Go(func() {
			for link := range linksCh {
				if ctx.Err() != nil {
					continue
				}

				page, nextLinks, ok := crw.fetchPage(ctx, link.URL, link.depth)
				if ok {
					results <- crawlResult{page, nextLinks}
				}
			}
		})
	}

	for _, link := range level {
		linksCh <- link
	}
	close(linksCh)
	wg.Wait()
	close(results)

	levelResults := make([]crawlResult, 0, len(level))
	for result := range results {
		levelResults = append(levelResults, result)
	}

	return levelResults
}

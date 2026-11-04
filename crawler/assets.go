package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// AssetType identifies the kind of a resource referenced by a page.
type AssetType string

const (
	assetTypeImage  AssetType = "image"
	assetTypeScript AssetType = "script"
	assetTypeStyle  AssetType = "style"
)

// Asset contains the measured result for a resource referenced by a page.
type Asset struct {
	URL        string    `json:"url"`
	Type       AssetType `json:"type"`
	StatusCode int       `json:"status_code"`
	SizeBytes  int64     `json:"size_bytes"`
	Error      string    `json:"error,omitempty"`
}

type assetReference struct {
	URL  *url.URL
	Type AssetType
}

type assetCacheEntry struct {
	ready chan struct{}
	asset *Asset
}

func (crw *crawler) fetchAssets(ctx context.Context, doc *goquery.Document, pageURL *url.URL) []Asset {
	refs := extractAssets(doc, pageURL)
	assets := make([]Asset, 0, len(refs))

	for _, ref := range refs {
		if asset := crw.fetchAsset(ctx, ref); asset != nil {
			assets = append(assets, *asset)
		}
	}

	return assets
}

func (crw *crawler) fetchAsset(ctx context.Context, ref assetReference) *Asset {
	key := ref.URL.String()

	for ctx.Err() == nil {
		entry, isLoadOwner := crw.beginAssetLoad(key)
		if isLoadOwner {
			asset := crw.loadAssetOnce(ctx, ref)
			crw.finishAssetLoad(key, entry, asset)
			return asset
		}

		cached, err := waitForAsset(ctx, entry)
		if err != nil {
			return nil
		}
		if cached == nil {
			continue
		}

		cached.Type = ref.Type

		return cached
	}

	return nil
}

// beginAssetLoad returns the cache entry and whether this caller owns its load.
func (crw *crawler) beginAssetLoad(key string) (*assetCacheEntry, bool) {
	crw.assetCacheMu.Lock()
	defer crw.assetCacheMu.Unlock()

	if entry, exists := crw.assetCache[key]; exists {
		return entry, false
	}

	entry := &assetCacheEntry{ready: make(chan struct{})}
	crw.assetCache[key] = entry

	return entry, true
}

// finishAssetLoad publishes the result and wakes waiters; cancelled loads are removed.
func (crw *crawler) finishAssetLoad(key string, entry *assetCacheEntry, asset *Asset) {
	crw.assetCacheMu.Lock()
	defer crw.assetCacheMu.Unlock()

	entry.asset = asset
	if asset == nil {
		delete(crw.assetCache, key)
	}
	close(entry.ready)
}

// waitForAsset waits for the shared load or for this caller's context to be cancelled.
func waitForAsset(ctx context.Context, entry *assetCacheEntry) (*Asset, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-entry.ready:
		if entry.asset == nil {
			return nil, nil
		}

		asset := *entry.asset

		return &asset, nil
	}
}

func (crw *crawler) loadAssetOnce(ctx context.Context, ref assetReference) *Asset {
	asset := &Asset{URL: ref.URL.String(), Type: ref.Type}

	resp, err := crw.fetcher.get(ctx, ref.URL)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}

		asset.Error = err.Error()

		return asset
	}
	defer func() { _ = resp.Body.Close() }()

	asset.StatusCode = resp.StatusCode
	if resp.StatusCode >= http.StatusBadRequest {
		asset.Error = fmt.Sprintf("HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	size, err := readAssetSize(resp)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}

		if asset.Error != "" {
			asset.Error += "; "
		}
		asset.Error += err.Error()
	} else {
		asset.SizeBytes = size
	}

	return asset
}

func readAssetSize(resp *http.Response) (int64, error) {
	if resp.ContentLength >= 0 {
		return resp.ContentLength, nil
	}

	n, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read response body: %w", err)
	}

	return n, nil
}

func extractAssets(doc *goquery.Document, pageURL *url.URL) []assetReference {
	var assets []assetReference
	seen := make(map[string]struct{})

	doc.Find("img[src], script[src], link[href]").Each(func(_ int, selection *goquery.Selection) {
		reference, ok := extractAssetReference(selection, pageURL)
		if !ok {
			return
		}

		key := reference.URL.String()
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		assets = append(assets, reference)
	})

	slices.SortStableFunc(assets, func(a, b assetReference) int {
		return strings.Compare(string(a.Type), string(b.Type))
	})

	return assets
}

func extractAssetReference(sel *goquery.Selection, pageURL *url.URL) (assetReference, bool) {
	var (
		rawURL string
		typ    AssetType
	)

	switch sel.Get(0).Data {
	case "img":
		rawURL, typ = sel.AttrOr("src", ""), assetTypeImage
	case "script":
		rawURL, typ = sel.AttrOr("src", ""), assetTypeScript
	case "link":
		if !hasRelToken(sel.AttrOr("rel", ""), "stylesheet") {
			return assetReference{}, false
		}
		rawURL, typ = sel.AttrOr("href", ""), assetTypeStyle
	default:
		return assetReference{}, false
	}

	absoluteURL, ok := resolveURL(pageURL, rawURL)
	if !ok {
		return assetReference{}, false
	}

	return assetReference{URL: absoluteURL, Type: typ}, true
}

func hasRelToken(rel, token string) bool {
	return slices.ContainsFunc(strings.Fields(rel), func(v string) bool {
		return strings.EqualFold(v, token)
	})
}

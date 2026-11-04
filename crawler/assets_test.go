package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/require"
)

type observedDoneContext struct {
	context.Context
	called chan struct{}
	once   sync.Once
}

func (ctx *observedDoneContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.called) })

	return ctx.Context.Done()
}

func waitSignal[T any](t *testing.T, ch <-chan T, msg string) T {
	t.Helper()

	select {
	case v := <-ch:
		return v
	case <-time.After(3 * time.Second):
		t.Fatal(msg)
		var zero T

		return zero
	}
}

func TestExtractAssets(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`
		<html>
			<head>
				<link rel="alternate stylesheet" href="/css/site.css">
				<link rel="icon" href="/favicon.ico">
				<script src="https://cdn.example.test/app.js"></script>
			</head>
			<body>
				<img src="../images/logo.png">
				<img src="../images/logo.png">
				<script src="data:text/javascript,alert(1)"></script>
			</body>
		</html>
	`))
	require.NoError(t, err)

	pageURL, err := url.Parse("https://example.test/docs/index.html")
	require.NoError(t, err)

	got := extractAssets(doc, pageURL)
	gotAssets := make([]Asset, len(got))
	for i, asset := range got {
		gotAssets[i] = Asset{URL: asset.URL.String(), Type: asset.Type}
	}

	require.Equal(t, []Asset{
		{URL: "https://example.test/images/logo.png", Type: assetTypeImage},
		{URL: "https://cdn.example.test/app.js", Type: assetTypeScript},
		{URL: "https://example.test/css/site.css", Type: assetTypeStyle},
	}, gotAssets)
}

func TestAssetJSONOmitsEmptyError(t *testing.T) {
	data, err := json.Marshal(Asset{})
	require.NoError(t, err)
	require.JSONEq(t, `{"url":"","type":"","status_code":0,"size_bytes":0}`, string(data))
}

func TestPageAssetsIncludeStatusAndSize(t *testing.T) {
	externalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "external js")
	}))
	t.Cleanup(externalServer.Close)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = fmt.Fprintf(w, `
				<img src="/logo.png">
				<script src="%s/app.js"></script>
				<link rel="stylesheet" href="/missing.css">
			`, externalServer.URL)
		case "/logo.png":
			_, _ = fmt.Fprint(w, "123456")
		case "/missing.css":
			http.Error(w, "missing", http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	data, err := Analyze(t.Context(), Options{
		URL:        server.URL,
		HTTPClient: server.Client(),
	})
	require.NoError(t, err)

	var report Report
	require.NoError(t, json.Unmarshal(data, &report))
	require.Len(t, report.Pages, 1)
	require.Equal(t, []Asset{
		{
			URL:        server.URL + "/logo.png",
			Type:       assetTypeImage,
			StatusCode: http.StatusOK,
			SizeBytes:  6,
		},
		{
			URL:        externalServer.URL + "/app.js",
			Type:       assetTypeScript,
			StatusCode: http.StatusOK,
			SizeBytes:  11,
		},
		{
			URL:        server.URL + "/missing.css",
			Type:       assetTypeStyle,
			StatusCode: http.StatusNotFound,
			SizeBytes:  int64(len("missing\n")),
			Error:      "HTTP 404 Not Found",
		},
	}, report.Pages[0].Assets)
}

func TestAssetSizeWithoutContentLength(t *testing.T) {
	const body = "chunked asset body"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			_, _ = fmt.Fprint(w, `<img src="/stream">`)
			return
		}

		w.(http.Flusher).Flush()
		_, _ = fmt.Fprint(w, body)
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	data, err := Analyze(t.Context(), Options{
		URL:        server.URL,
		HTTPClient: server.Client(),
	})
	require.NoError(t, err)

	var report Report
	require.NoError(t, json.Unmarshal(data, &report))
	require.Len(t, report.Pages, 1)
	require.Equal(t, []Asset{{
		URL:        server.URL + "/stream",
		Type:       assetTypeImage,
		StatusCode: http.StatusOK,
		SizeBytes:  int64(len(body)),
	}}, report.Pages[0].Assets)
}

func TestPageWithoutAssetsHasEmptyArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `<p>no assets</p>`)
	}))
	t.Cleanup(server.Close)

	data, err := Analyze(t.Context(), Options{URL: server.URL, HTTPClient: server.Client()})
	require.NoError(t, err)

	var report Report
	require.NoError(t, json.Unmarshal(data, &report))
	require.Len(t, report.Pages, 1)
	require.NotNil(t, report.Pages[0].BrokenLinks)
	require.Empty(t, report.Pages[0].BrokenLinks)
	require.NotNil(t, report.Pages[0].Assets)
	require.Empty(t, report.Pages[0].Assets)
}

func TestFetchAssetWaitsForInProgressLoad(t *testing.T) {
	var sharedRequests atomic.Int32
	sharedStarted := make(chan struct{})
	releaseShared := make(chan struct{})
	release := sync.OnceFunc(func() { close(releaseShared) })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if sharedRequests.Add(1) == 1 {
			close(sharedStarted)
		}
		<-releaseShared
		_, _ = fmt.Fprint(w, "shared asset")
	}))
	t.Cleanup(func() {
		release()
		server.Close()
	})

	crw, err := newCrawler(Options{URL: server.URL, HTTPClient: server.Client()})
	require.NoError(t, err)
	assetURL, err := url.Parse(server.URL + "/shared.png")
	require.NoError(t, err)

	firstDone := make(chan *Asset, 1)
	go func() {
		firstDone <- crw.fetchAsset(t.Context(), assetReference{
			URL:  assetURL,
			Type: assetTypeImage,
		})
	}()
	waitSignal(t, sharedStarted, "shared asset request did not start")

	secondContext := &observedDoneContext{
		Context: t.Context(),
		called:  make(chan struct{}),
	}
	secondDone := make(chan *Asset, 1)
	go func() {
		secondDone <- crw.fetchAsset(secondContext, assetReference{
			URL:  assetURL,
			Type: assetTypeScript,
		})
	}()
	waitSignal(t, secondContext.called, "second caller did not wait for the in-progress load")
	release()

	first := waitSignal(t, firstDone, "first asset request did not finish")
	second := waitSignal(t, secondDone, "second asset request did not finish")
	require.Equal(t, int32(1), sharedRequests.Load())
	require.Equal(t, &Asset{
		URL:        assetURL.String(),
		Type:       assetTypeImage,
		StatusCode: http.StatusOK,
		SizeBytes:  int64(len("shared asset")),
	}, first)
	require.Equal(t, &Asset{
		URL:        assetURL.String(),
		Type:       assetTypeScript,
		StatusCode: http.StatusOK,
		SizeBytes:  int64(len("shared asset")),
	}, second)
}

func TestAssetSharedAcrossPagesIsRequestedOnce(t *testing.T) {
	var sharedRequests atomic.Int32
	var secondPageRequests atomic.Int32
	sharedStarted := make(chan struct{})
	releaseShared := make(chan struct{})
	release := sync.OnceFunc(func() { close(releaseShared) })
	secondPageFetchStarted := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = fmt.Fprint(w, `
				<a href="/a">a</a>
				<a href="/second">second</a>
				<a href="/c">c</a>
			`)
		case "/a":
			_, _ = fmt.Fprint(w, `<img src="/shared.png">`)
		case "/second":
			if secondPageRequests.Add(1) == 2 {
				close(secondPageFetchStarted)
			}
			_, _ = fmt.Fprint(w, `<script src="/shared.png"></script>`)
		case "/c":
			_, _ = fmt.Fprint(w, `<link rel="stylesheet" href="/shared.png">`)
		case "/shared.png":
			if sharedRequests.Add(1) == 1 {
				close(sharedStarted)
			}
			<-releaseShared
			_, _ = fmt.Fprint(w, "shared asset")
		default:
			http.NotFound(w, r)
		}
	})
	server := httptest.NewServer(handler)
	t.Cleanup(func() {
		release()
		server.Close()
	})

	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		data, err := Analyze(t.Context(), Options{
			URL:         server.URL,
			HTTPClient:  server.Client(),
			Depth:       1,
			Concurrency: 3,
		})
		done <- result{data: data, err: err}
	}()

	waitSignal(t, sharedStarted, "shared asset request did not start")

	waitSignal(t, secondPageFetchStarted, "second page fetch did not start while shared asset was pending")
	release()

	got := waitSignal(t, done, "crawl did not finish")
	require.NoError(t, got.err)
	require.Equal(t, int32(1), sharedRequests.Load())

	var report Report
	require.NoError(t, json.Unmarshal(got.data, &report))
	require.Len(t, report.Pages, 4)
	require.Equal(t, []Asset{{
		URL:        server.URL + "/shared.png",
		Type:       assetTypeImage,
		StatusCode: http.StatusOK,
		SizeBytes:  int64(len("shared asset")),
	}}, report.Pages[1].Assets)
	require.Equal(t, []Asset{{
		URL:        server.URL + "/shared.png",
		Type:       assetTypeStyle,
		StatusCode: http.StatusOK,
		SizeBytes:  int64(len("shared asset")),
	}}, report.Pages[2].Assets)
	require.Equal(t, []Asset{{
		URL:        server.URL + "/shared.png",
		Type:       assetTypeScript,
		StatusCode: http.StatusOK,
		SizeBytes:  int64(len("shared asset")),
	}}, report.Pages[3].Assets)
}

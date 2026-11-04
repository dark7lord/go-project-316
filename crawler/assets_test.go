package crawler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"code/crawler"
)

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

func TestPageAssetsIncludeStatusAndSize(t *testing.T) {
	externalServer := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "external js")
	}))

	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))

	report := analyzeReport(t, crawler.Options{
		URL:        server.URL,
		Depth:      1,
		HTTPClient: server.Client(),
	})
	require.Len(t, report.Pages, 1)
	require.Equal(t, []crawler.Asset{
		{
			URL:        server.URL + "/logo.png",
			Type:       "image",
			StatusCode: http.StatusOK,
			SizeBytes:  6,
		},
		{
			URL:        externalServer.URL + "/app.js",
			Type:       "script",
			StatusCode: http.StatusOK,
			SizeBytes:  11,
		},
		{
			URL:        server.URL + "/missing.css",
			Type:       "style",
			StatusCode: http.StatusNotFound,
			SizeBytes:  int64(len("missing\n")),
			Error:      "HTTP 404 Not Found",
		},
	}, report.Pages[0].Assets)
}

func TestAssetSizeWithoutContentLength(t *testing.T) {
	const body = "chunked asset body"
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			_, _ = fmt.Fprint(w, `<img src="/stream">`)
			return
		}

		w.(http.Flusher).Flush()
		_, _ = fmt.Fprint(w, body)
	}))

	report := analyzeReport(t, crawler.Options{
		URL:        server.URL,
		Depth:      1,
		HTTPClient: server.Client(),
	})
	require.Len(t, report.Pages, 1)
	require.Equal(t, []crawler.Asset{{
		URL:        server.URL + "/stream",
		Type:       "image",
		StatusCode: http.StatusOK,
		SizeBytes:  int64(len(body)),
	}}, report.Pages[0].Assets)
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
	server := newTestServer(t, handler)
	t.Cleanup(release)

	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		data, err := crawler.Analyze(t.Context(), crawler.Options{
			URL:         server.URL,
			HTTPClient:  server.Client(),
			Depth:       2,
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

	var report crawler.Report
	require.NoError(t, json.Unmarshal(got.data, &report))
	require.Len(t, report.Pages, 4)
	require.Equal(t, []crawler.Asset{{
		URL:        server.URL + "/shared.png",
		Type:       "image",
		StatusCode: http.StatusOK,
		SizeBytes:  int64(len("shared asset")),
	}}, report.Pages[1].Assets)
	require.Equal(t, []crawler.Asset{{
		URL:        server.URL + "/shared.png",
		Type:       "style",
		StatusCode: http.StatusOK,
		SizeBytes:  int64(len("shared asset")),
	}}, report.Pages[2].Assets)
	require.Equal(t, []crawler.Asset{{
		URL:        server.URL + "/shared.png",
		Type:       "script",
		StatusCode: http.StatusOK,
		SizeBytes:  int64(len("shared asset")),
	}}, report.Pages[3].Assets)
}

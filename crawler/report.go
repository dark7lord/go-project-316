package crawler

import (
	"encoding/json"
	"strings"
	"time"
)

// BrokenLink is a link found on a page and its check result.
type BrokenLink struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Report is the crawl report.
type Report struct {
	RootURL     string    `json:"root_url"`
	Depth       uint      `json:"depth"`
	GeneratedAt time.Time `json:"generated_at"`
	Pages       []Page    `json:"pages"`
}

func (crw *crawler) report() (data []byte, err error) {
	result := Report{
		RootURL:     crw.rootURL.String(),
		Depth:       crw.depth,
		Pages:       crw.pages,
		GeneratedAt: time.Now().UTC(),
	}

	indent := strings.Repeat(" ", int(crw.indentJSON))

	return json.MarshalIndent(result, "", indent)
}

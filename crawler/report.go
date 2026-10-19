package crawler

import (
	"encoding/json"
	"strings"
	"time"
)

// Page is a single crawled page in the report.
type Page struct {
	URL          string    `json:"url"`
	Depth        uint      `json:"depth"`
	HTTPStatus   int       `json:"http_status"`
	Status       string    `json:"status"`
	Error        string    `json:"error"`
	BrokenLinks  []Link    `json:"broken_links,omitempty"`
	DiscoveredAt time.Time `json:"discovered_at"`
}

// Link is a link found on a page and its check result.
type Link struct {
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

func report(crw crawler, depth uint, indentJSON uint) (data []byte, err error) {
	// TODO: что же делать с глубиной и как понять что это корень?

	result := Report{
		RootURL:     crw.rootURL.String(),
		Depth:       depth,
		GeneratedAt: time.Now().UTC(),
		Pages:       crw.pages,
	}

	indent := strings.Repeat(" ", int(indentJSON))

	return json.MarshalIndent(result, "", indent)
}

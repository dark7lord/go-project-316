package crawler

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// SEOResult holds basic SEO data of a page.
type SEOResult struct {
	HasTitle       bool   `json:"has_title"`
	Title          string `json:"title"`
	HasDescription bool   `json:"has_description"`
	Description    string `json:"description"`
	HasH1          bool   `json:"has_h1"`
}

func extractSEO(doc *goquery.Document) SEOResult {
	title := doc.Find("head > title").First()
	desc := doc.Find(`meta[name="description"]`).First()
	content, _ := desc.Attr("content")

	return SEOResult{
		HasTitle:       title.Length() > 0,
		Title:          strings.TrimSpace(title.Text()),
		HasDescription: desc.Length() > 0,
		Description:    strings.TrimSpace(content),
		HasH1:          doc.Find("h1").Length() > 0,
	}
}

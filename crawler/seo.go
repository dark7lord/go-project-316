package crawler

import (
	"slices"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// SEOResult holds basic SEO data of a page.
type SEOResult struct {
	HasTitle       bool   `json:"has_title"`
	Title          string `json:"title,omitempty"`
	HasDescription bool   `json:"has_description"`
	Description    string `json:"description,omitempty"`
	HasH1          bool   `json:"has_h1"`
}

func titleText(node *html.Node) string {
	if node.FirstChild == nil {
		return ""
	}
	data := node.FirstChild.Data
	title := strings.TrimSpace(data)

	return title
}

func descriptionText(node *html.Node) string {
	idxName := slices.IndexFunc(node.Attr, func(attr html.Attribute) bool {
		return attr.Key == "name" && attr.Val == "description"
	})
	if idxName < 0 {
		return ""
	}

	idxContent := slices.IndexFunc(node.Attr, func(attr html.Attribute) bool {
		return attr.Key == "content"
	})
	if idxContent < 0 {
		return ""
	}

	value := node.Attr[idxContent].Val
	description := strings.TrimSpace(value)

	return description
}

func extractSEO(doc *html.Node) SEOResult {
	var result SEOResult

	for node := range doc.Descendants() {
		if node.Type != html.ElementNode {
			continue
		}

		switch node.DataAtom {
		case atom.Title:
			result.Title = titleText(node)
			result.HasTitle = result.Title != ""
		case atom.Meta:
			if desc := descriptionText(node); desc != "" {
				result.Description = desc
				result.HasDescription = true
			}
		case atom.H1:
			result.HasH1 = true
		}
	}

	return result
}

// Package crawler implements website structure analysis.
package crawler

import "context"

// Options configures a crawl run.
type Options struct {
}

// Analyze crawls a website and returns a report about its structure.
func Analyze(ctx context.Context, opts Options) string {
	_ = ctx
	_ = opts

	return "analyze!"
}

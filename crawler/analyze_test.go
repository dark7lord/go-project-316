package crawler

import (
	"context"
	"testing"
)

func TestAnalyze(t *testing.T) {
	got := Analyze(context.Background(), Options{})
	if got != "analyze!" {
		t.Fatalf("Analyze() = %q, want %q", got, "analyze!")
	}
}

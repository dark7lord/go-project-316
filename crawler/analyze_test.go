package crawler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnalyze(t *testing.T) {
	got, err := Analyze(context.Background(), Options{})
	require.NoError(t, err)

	var result Result
	errJSON := json.Unmarshal(got, &result)
	require.NoError(t, errJSON)
	require.Len(t, result.Pages, 1)
	require.Contains(t, result.Pages[0].Error, "panic")
}

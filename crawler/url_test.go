package crawler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"scheme-less domain", "ya.ru", "http://ya.ru", false},
		{"https passthrough", "https://ya.ru/path", "https://ya.ru/path", false},
		{"http passthrough", "http://ya.ru/foo", "http://ya.ru/foo", false},
		{"scheme-less localhost with port", "localhost:8080/x", "http://localhost:8080/x", false},
		{"scheme-less local ip with port", "127.0.0.1:3000", "http://127.0.0.1:3000", false},
		{"http localhost", "http://localhost:3000/x", "http://localhost:3000/x", false},
		{"path preserved", "example.com/foo?x=1", "http://example.com/foo?x=1", false},
		{"surrounding spaces are trimmed", "  ya.ru  ", "http://ya.ru", false},
		{"unsupported scheme", "ftp://x", "", true},
		{"javascript scheme", "javascript:alert(1)", "", true},
		{"empty", "", "", true},
		{"whitespace only", "   ", "", true},
		{"no host", "http://", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeURL(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

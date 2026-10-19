package crawler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"scheme-less domain", "ya.ru", "http://ya.ru"},
		{"https passthrough", "https://ya.ru/path", "https://ya.ru/path"},
		{"http passthrough", "http://ya.ru/foo", "http://ya.ru/foo"},
		{"scheme-less localhost with port", "localhost:8080/x", "http://localhost:8080/x"},
		{"scheme-less local ip with port", "127.0.0.1:3000", "http://127.0.0.1:3000"},
		{"path and query preserved", "example.com/foo?x=1", "http://example.com/foo?x=1"},
		{"surrounding spaces are trimmed", "  ya.ru  ", "http://ya.ru"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeURL(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.want, got.String())
		})
	}
}

func TestNormalizeURLInvalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unsupported scheme", "ftp://x"},
		{"javascript scheme", "javascript:alert(1)"},
		{"empty", ""},
		{"whitespace only", "   "},
		{"no host", "http://"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeURL(tt.input)
			require.ErrorIs(t, err, ErrInvalidURL)
		})
	}
}

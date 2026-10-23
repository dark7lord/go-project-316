package app

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResolveDelayFlags(t *testing.T) {
	tests := []struct {
		name  string
		delay time.Duration
		rps   uint
		want  time.Duration
	}{
		{
			name: "no flags",
			want: 0,
		},
		{
			name:  "only delay flag",
			delay: 200 * time.Millisecond,
			want:  200 * time.Millisecond,
		},
		{
			name: "only rps flag",
			rps:  5,
			want: 200 * time.Millisecond,
		},
		{
			name:  "delay and rps",
			delay: 10 * time.Millisecond,
			rps:   4,
			want:  250 * time.Millisecond,
		},
		{
			name: "1 second",
			rps:  1,
			want: time.Second,
		},
		{
			name: "rps above 1 billion means no limit",
			rps:  2_000_000_000,
			want: 0,
		},
		{
			name: "rps overflowing duration means no limit",
			rps:  math.MaxUint,
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveDelay(tt.rps, tt.delay)
			require.Equal(t, tt.want, got)
		})
	}
}

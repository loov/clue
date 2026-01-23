package build

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "450ms",
			duration: 450 * time.Millisecond,
			want:     "450ms",
		},
		{
			name:     "1ms",
			duration: 1 * time.Millisecond,
			want:     "1ms",
		},
		{
			name:     "999ms",
			duration: 999 * time.Millisecond,
			want:     "999ms",
		},
		{
			name:     "1.0s",
			duration: 1 * time.Second,
			want:     "1.0s",
		},
		{
			name:     "2.3s",
			duration: 2300 * time.Millisecond,
			want:     "2.3s",
		},
		{
			name:     "59.9s",
			duration: 59900 * time.Millisecond,
			want:     "59.9s",
		},
		{
			name:     "1m 0s",
			duration: 60 * time.Second,
			want:     "1m 0s",
		},
		{
			name:     "1m 12s",
			duration: 72 * time.Second,
			want:     "1m 12s",
		},
		{
			name:     "2m 30s",
			duration: 150 * time.Second,
			want:     "2m 30s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

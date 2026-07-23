package main

import (
	"testing"
	"time"
)

func TestNextRevalidationCheck(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "before 2 AM schedules today",
			now:  time.Date(2026, time.July, 23, 1, 0, 0, 0, time.UTC),
			want: time.Date(2026, time.July, 23, 2, 0, 0, 0, time.UTC),
		},
		{
			name: "after 2 AM schedules tomorrow",
			now:  time.Date(2026, time.July, 23, 3, 0, 0, 0, time.UTC),
			want: time.Date(2026, time.July, 24, 2, 0, 0, 0, time.UTC),
		},
		{
			name: "just after 2 AM schedules tomorrow",
			now:  time.Date(2026, time.July, 23, 2, 0, 1, 0, time.UTC),
			want: time.Date(2026, time.July, 24, 2, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextRevalidationCheck(tt.now)
			if !got.Equal(tt.want) {
				t.Fatalf("nextRevalidationCheck(%s) = %s, want %s", tt.now, got, tt.want)
			}
		})
	}
}

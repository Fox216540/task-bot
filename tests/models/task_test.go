package models_test

import (
	"testing"

	"task-bot/internal/models"
)

func TestBufferByType(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"deep", 15},
		{"medium", 10},
		{"light", 5},
		{"quick", 2},
		{"c", 15},
		{"м", 10},
		{"unknown", 5},
	}

	for _, tc := range cases {
		got := models.BufferByType(tc.in)
		if got != tc.want {
			t.Fatalf("BufferByType(%q)=%d, want %d", tc.in, got, tc.want)
		}
	}
}

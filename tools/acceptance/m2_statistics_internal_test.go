package main

import (
	"strings"
	"testing"
	"time"
)

func TestM2TimingSummary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		samples []m2Sample
		want    string
		fail    bool
	}{
		{
			name: "near measured distribution",
			samples: []m2Sample{
				{"pre-commit", "warm", 5, 0},
				{"pre-commit", "warm", 1, 0},
				{"pre-commit", "warm", 3, 0},
				{"pre-commit", "warm", 2, 0},
				{"pre-commit", "warm", 4, 0},
			},
			want: "median_ms=3 max_ms=5",
		},
		{name: "red absent measurements", fail: true},
		{"corrupted duration", []m2Sample{{"pre-commit", "cold", -1, 0}}, "", true},
		{"zero retained", []m2Sample{{"commit-msg", "warm", 0, 0}}, "median_ms=0 max_ms=0", false},
		{"unknown rung", []m2Sample{{"unknown", "warm", 1, 0}}, "", true},
		{"unknown cache", []m2Sample{{"pre-commit", "unknown", 1, 0}}, "", true},
		{"failed commit", []m2Sample{{"pre-commit", "warm", 1, 1}}, "", true},
		{"invalid status", []m2Sample{{"pre-push", "warm", 1, -1}}, "", true},
		{"measured push refusal", []m2Sample{{"pre-push", "warm", 1, 1}}, "median_ms=1 max_ms=1", false},
	}
	for _, tt := range tests {
		got, err := summarizeM2(tt.samples)
		if (err != nil) != tt.fail {
			t.Fatalf("%s: error=%v", tt.name, err)
		}
		if !tt.fail && !strings.Contains(got, tt.want) {
			t.Fatalf("%s: summary=%q", tt.name, got)
		}
	}
}

func TestM2WarmBudget(t *testing.T) {
	t.Parallel()
	warm := func(rung string, milliseconds ...int64) []m2Sample {
		samples := make([]m2Sample, 0, len(milliseconds))
		for _, elapsed := range milliseconds {
			samples = append(samples, m2Sample{Rung: rung, Cache: "warm", Milliseconds: elapsed})
		}
		return samples
	}
	tests := []struct {
		name    string
		samples []m2Sample
		want    string
		fail    bool
	}{
		{"near within the budget", warm("pre-commit", 1000, 9999, 5000), "pre-commit warm_median_ms=5000 budget_ms=10000 within", false},
		{"near at the budget", warm("commit-msg", 10000), "commit-msg warm_median_ms=10000 budget_ms=10000 within", false},
		{"red over the budget", warm("pre-push", 12000, 11000, 13000), "pre-push warm_median_ms=12000 budget_ms=10000 over", true},
		{"cold only", []m2Sample{{"pre-commit", "cold", 1, 0}}, "", true},
		{"red absent measurements", nil, "", true},
	}
	for _, tt := range tests {
		got, err := budgetM2(tt.samples, 10*time.Second)
		if (err != nil) != tt.fail {
			t.Fatalf("%s: error=%v", tt.name, err)
		}
		if !strings.Contains(got, tt.want) {
			t.Fatalf("%s: comparison=%q", tt.name, got)
		}
	}
}

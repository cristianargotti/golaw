package main

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestM2StatisticsPreservesRawSamplesAndOrder(t *testing.T) {
	t.Parallel()
	samples := []m2Sample{
		{"pre-push", "warm", 0, 2},
		{"commit-msg", "cold", 6, 0},
		{"pre-commit", "warm", 9, 0},
		{"pre-commit", "cold", 7, 0},
		{"pre-commit", "warm", 1, 0},
		{"pre-commit", "warm", 8, 0},
		{"pre-push", "warm", 2, 127},
		{"pre-commit", "warm", 2, 0},
	}
	original := slices.Clone(samples)
	got, err := summarizeM2(samples)
	want := strings.Join([]string{
		"pre-commit cold samples_ms=[7] codes=[0] count=1 median_ms=7 max_ms=7",
		"pre-commit warm samples_ms=[9 1 8 2] codes=[0 0 0 0] count=4 median_ms=8 max_ms=9",
		"commit-msg cold samples_ms=[6] codes=[0] count=1 median_ms=6 max_ms=6",
		"pre-push warm samples_ms=[0 2] codes=[2 127] count=2 median_ms=2 max_ms=2",
	}, "\n")
	if err != nil || got != want {
		t.Fatalf("summary=%q error=%v, want %q", got, err, want)
	}
	if !slices.Equal(samples, original) {
		t.Fatal("summary changed its input samples")
	}
}

func TestM2StatisticsBudgetNamesEveryFailure(t *testing.T) {
	t.Parallel()
	samples := []m2Sample{
		{"pre-push", "cold", 0, 1},
		{"commit-msg", "warm", 14, 0},
		{"pre-commit", "warm", 11, 0},
		{"pre-commit", "warm", 1, 0},
	}
	original := slices.Clone(samples)
	got, err := budgetM2(samples, 10*time.Millisecond)
	want := "pre-commit warm_median_ms=11 budget_ms=10 over\ncommit-msg warm_median_ms=14 budget_ms=10 over"
	if !errors.Is(err, ErrAcceptance) || got != want {
		t.Fatalf("comparison=%q error=%v, want %q and acceptance error", got, err, want)
	}
	for _, name := range []string{"pre-commit", "commit-msg", "pre-push"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("budget error omits %s: %v", name, err)
		}
	}
	if !slices.Equal(samples, original) {
		t.Fatal("budget comparison changed its input samples")
	}
}

func TestM2StatisticsValidatesEverySample(t *testing.T) {
	t.Parallel()
	invalid := []m2Sample{
		{"pre-commit", "warm", -1, 0},
		{"pre-commit", "warm", 0, -1},
		{"pre-commit", "warm", 0, 1},
		{"commit-msg", "cold", 1, 2},
		{"pre-push", "warm", 1, -1},
		{"pre-push", "unknown", 1, 0},
		{"unknown", "warm", 1, 0},
	}
	for _, sample := range invalid {
		samples := []m2Sample{{"pre-commit", "warm", 0, 0}, sample}
		if _, err := summarizeM2(samples); !errors.Is(err, ErrAcceptance) {
			t.Fatalf("summary accepted invalid sample %+v: %v", sample, err)
		}
		if _, err := budgetM2(samples, time.Second); !errors.Is(err, ErrAcceptance) {
			t.Fatalf("budget accepted invalid sample %+v: %v", sample, err)
		}
	}
	for _, summarize := range []func([]m2Sample) (string, error){
		summarizeM2,
		func(samples []m2Sample) (string, error) { return budgetM2(samples, time.Second) },
	} {
		if _, err := summarize(nil); !errors.Is(err, ErrAcceptance) {
			t.Fatalf("empty input lacks acceptance error: %v", err)
		}
	}
}

func TestM2StatisticsBudgetRetainsBoundaryAndLargeDurations(t *testing.T) {
	t.Parallel()
	for _, milliseconds := range []int64{0, 1, 1<<63 - 1} {
		samples := []m2Sample{{"pre-push", "cold", 1<<63 - 1, 255}, {"pre-push", "warm", milliseconds, 255}}
		summary, err := summarizeM2(samples)
		if err != nil || !strings.Contains(summary, "9223372036854775807") {
			t.Fatalf("raw duration overflowed: %q %v", summary, err)
		}
		comparison, err := budgetM2(samples, time.Millisecond)
		if (err != nil) != (milliseconds > 1) || !strings.Contains(comparison, "budget_ms=1 ") {
			t.Fatalf("milliseconds=%d comparison=%q error=%v", milliseconds, comparison, err)
		}
	}
}

package usecase

import (
	"slices"
	"testing"
)

func TestWordsSplitAGoNameTheWayItReads(t *testing.T) {
	t.Parallel()
	tests := map[string][]string{
		"ReadPlan":      {"read", "plan"},
		"HTTPGet":       {"http", "get"},
		"GetHTTP":       {"get", "http"},
		"ArchivePlanV2": {"archive", "plan", "v2"},
		"GetV2Plan":     {"get", "v2", "plan"},
		"Plan":          {"plan"},
	}
	for in, want := range tests {
		if got := words(in); !slices.Equal(got, want) {
			t.Errorf("words(%q) = %v, want %v", in, got, want)
		}
	}
	if got := snake("ReadPlan"); got != "read_plan" {
		t.Errorf("snake(ReadPlan) = %q", got)
	}
}

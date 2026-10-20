package main

import (
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/doctor"
)

func TestSetupLinesBelongToTheRepositoryOnce(t *testing.T) {
	t.Parallel()
	base := []doctor.Report{
		{Schema: 1, Lines: []doctor.Line{{ID: "doctor.plugin", Key: "first"}}},
		{Schema: 1, Lines: []doctor.Line{{ID: "doctor.plugin", Key: "second"}}},
	}
	lines := []doctor.Line{{ID: "doctor.setup-receipt", State: "not-armed"}}
	for _, tc := range []struct {
		name  string
		count int
	}{
		{"one agent", 1},
		{"two agents", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input := base[:tc.count]
			got := withSetupLines(input, lines)
			if len(got) != tc.count {
				t.Fatalf("report count = %d", len(got))
			}
			requireSetupOnce(t, got)
			requireUnmutated(t, input, lines)
		})
	}
}

// requireSetupOnce requires the setup line once, in the first report.
func requireSetupOnce(t *testing.T, got []doctor.Report) {
	t.Helper()
	found := 0
	for index, report := range got {
		for _, line := range report.Lines {
			if line.ID != "doctor.setup-receipt" {
				continue
			}
			found++
			if index != 0 {
				t.Fatal("setup lines attached to a later agent")
			}
		}
	}
	if found != 1 {
		t.Fatalf("setup report count = %d", found)
	}
}

// requireUnmutated requires the input reports and the setup lines as they
// were before the merge.
func requireUnmutated(t *testing.T, input []doctor.Report, lines []doctor.Line) {
	t.Helper()
	for _, report := range input {
		if len(report.Lines) != 1 {
			t.Fatal("input report mutated")
		}
	}
	if !reflect.DeepEqual(lines, []doctor.Line{{
		ID: "doctor.setup-receipt", State: "not-armed",
	}}) {
		t.Fatal("setup lines mutated")
	}
}

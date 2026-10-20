package main

import (
	"bytes"
	"context"
	"testing"
)

func TestM2CollectionCannotPassBrokenExecution(t *testing.T) {
	t.Parallel()
	for _, mode := range []struct {
		name string
		want string
	}{
		{"answer", statusPass},
		{"corrupt", statusFail},
		{"broken", statusFail},
	} {
		s := m2TestSuite(t)
		s.m2OS = "windows"
		script := m2CollectionScript(t, s, mode.name)
		s.execute = script.Execute
		rows, metadata := s.collectM2(t.Context())
		want := len(m2StagedNames()) + len(m2LifecycleNames()) + len(m2NativeNames())
		if len(rows) != want {
			t.Fatalf("M2 rows = %d, want %d", len(rows), want)
		}
		for _, item := range rows {
			if item.status != mode.want || !item.required {
				t.Fatalf("%s: row=%+v", mode.name, item)
			}
			if metadata[item.category+"/"+item.name].Agent != "git" {
				t.Fatalf("missing Git identity for %s", item.name)
			}
		}
		var out bytes.Buffer
		got := reportEvidence(&out, rows, "", s.digest, metadata)
		if got != (mode.want == statusPass) {
			t.Fatalf("%s: collection report=%v", mode.name, got)
		}
	}
}

func TestM2MeasurementRequiresCompletion(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		got  result
		want string
	}{
		{"red absent", result{}, statusFail},
		{"near complete", result{stdout: "M2_MEASUREMENTS_COMPLETE\n"}, statusPass},
		{"corrupted answer", result{stdout: "PASS\n"}, statusFail},
		{"skipped", result{stdout: "SKIP\n"}, statusFail},
		{"failed status", result{stdout: "M2_MEASUREMENTS_COMPLETE\n", code: 1}, statusFail},
		{"broken execution", result{err: ErrAcceptance}, statusFail},
	} {
		s := m2TestSuite(t)
		calls := 0
		s.execute = func(_ context.Context, c command) result {
			calls++
			if c.program != s.goTool {
				t.Fatalf("measurement used unexpected program %q", c.program)
			}
			return tt.got
		}
		item := s.measurementM2(t.Context())
		if calls != 1 || item.status != tt.want || !item.required {
			t.Fatalf("%s: calls=%d row=%+v", tt.name, calls, item)
		}
	}
}

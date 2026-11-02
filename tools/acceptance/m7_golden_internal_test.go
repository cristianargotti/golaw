package main

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/policy"
)

func TestM7GoldenEnvNamesTheToolsAndAsksForEveryTestLine(t *testing.T) {
	t.Parallel()
	env := m7GoldenEnv([]string{"GOFLAGS=-mod=mod", m7GoldenToolsKey + "=/old", "HOME=/h"}, "/tools")
	if environmentValue(env, m7GoldenToolsKey) != "/tools" || environmentValue(env, "GOFLAGS") != "-mod=mod -v" {
		t.Fatalf("env = %q", env)
	}
	if got := environmentValue(m7GoldenEnv([]string{"HOME=/h"}, "/tools"), "GOFLAGS"); got != "-v" {
		t.Fatalf("GOFLAGS = %q", got)
	}
}

func TestM7JudgeGoldenSkipsOnAMissingPin(t *testing.T) {
	t.Parallel()
	missing := "    golden_test.go:31: golangci-lint 2.14.0 " + m7SkipReason + "\n--- SKIP: TestGoldenPort/lint\nok\n"
	rows := []struct {
		name string
		got  result
		want string
	}{
		{"near every step ran", result{stdout: "=== RUN   TestGoldenPort\n--- PASS: TestGoldenPort\nok\n"}, "pass"},
		{"near a missing pin is a skip", result{stdout: missing}, "skip"},
		{"red a law broken", result{code: 2, stdout: "--- FAIL: TestGoldenPort/laws\n"}, "fail"},
		{"red a failing step that exited 0", result{stdout: "--- FAIL: TestGoldenAdapterSQL\n"}, "fail"},
		{"red the target did not run", result{err: ErrAcceptance}, "fail"},
	}
	for _, row := range rows {
		err := m7JudgeGolden(row.got)
		var skipped *m7Skipped
		got := "fail"
		switch {
		case err == nil:
			got = "pass"
		case errors.As(err, &skipped):
			got = "skip"
		}
		if got != row.want {
			t.Fatalf("%s: %s, %v", row.name, got, err)
		}
	}
}

// A failed step carries the lines its test logged, so a row that fails
// only under load names its cause in the record.
func TestM7JudgeGoldenKeepsTheOutputOfTheFailedStep(t *testing.T) {
	t.Parallel()
	got := result{code: 2, stdout: "=== RUN   TestGoldenHarnessNear\n--- FAIL: TestGoldenHarnessNear (19.78s)\n" +
		"    goldentest_test.go:36: greeter: lint: exit 1: timeout\n    goldentest_test.go:37: second line\nFAIL\n"}
	err := m7JudgeGolden(got)
	if err == nil || !strings.Contains(err.Error(), "greeter: lint: exit 1: timeout | goldentest_test.go:37: second line") {
		t.Fatalf("judge = %v, want the failed step's lines", err)
	}
}

func TestM7GoldenCasesAreInTheCatalog(t *testing.T) {
	t.Parallel()
	for _, catalog := range m7Catalogs() {
		if catalog.category == m7GoldenCategory {
			if !slices.Equal(m7Names(catalog.cases), []string{"kinds"}) || !catalog.cases[0].gen {
				t.Fatalf("m7-golden = %+v", catalog.cases)
			}
			return
		}
	}
	t.Fatal("m7Catalogs lacks m7-golden")
}

// TestM7GoldenKindsRunsTheTargetInTheKit: the row runs golden-gen in the
// kit checkout with the make of the M3 rows, names the tools directory over
// the shared data home and reads the run through m7JudgeGolden.
func TestM7GoldenKindsRunsTheTargetInTheKit(t *testing.T) {
	t.Parallel()
	s := newM7Suite(m2TestSuite(t))
	s.m2.Make = filepath.Join(s.base, "make")
	var seen []command
	s.execute = func(_ context.Context, c command) result {
		seen = append(seen, c)
		return result{stdout: "--- FAIL: TestGoldenPort/laws\n", code: 2}
	}
	scope := s.m7Scope(m7GoldenCategory, "kinds")
	if err := scope.m7GoldenKinds(t.Context()); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("a failing law answered %v", err)
	}
	want, err := policy.ToolsDir(runtime.GOOS, func(key string) string { return environmentValue(scope.m7GenEnv(), key) })
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0].program != s.m2.Make || !slices.Equal(seen[0].args, []string{"golden-gen"}) ||
		seen[0].dir != s.root || seen[0].timeout != m2MeasurementTimeout ||
		environmentValue(seen[0].env, m7GoldenToolsKey) != want {
		t.Fatalf("commands = %+v, tools %s", seen, want)
	}
}

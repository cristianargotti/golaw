package main

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen"
)

func TestM7CheckLinesAreTheContractShapes(t *testing.T) {
	t.Parallel()
	if got := m7DriftLine(m7FakePath, gen.DriftDiffers); got !=
		"golaw gen --check: drift internal/application/growth/ledgertest/zz_generated_fake.go differs from its rendering: run golaw gen" {
		t.Fatalf("drift line = %q", got)
	}
	if got := m7HoleLine("plans.read-plan"); got != " plans.read-plan: fill it and its red test" {
		t.Fatalf("hole line = %q", got)
	}
}

func TestM7JudgeCheckWantsTheExitAndEveryLine(t *testing.T) {
	t.Parallel()
	drift := m7DriftLine(m7PlansPath, gen.DriftMissing)
	rows := []struct {
		name string
		got  result
		code int
		want []string
		ok   bool
	}{
		{"near the drift named", result{code: 1, stdout: drift + "\n"}, 1, []string{drift}, true},
		{"near clean", result{stdout: m7CheckClean + "\n"}, 0, []string{m7CheckClean}, true},
		{"red the drift unnamed", result{code: 1, stdout: "golaw gen --check: 1 drift\n"}, 1, []string{drift}, false},
		{"red clean while drifting", result{stdout: m7CheckClean + "\n"}, 1, []string{drift}, false},
		{"red unmeasured", result{code: 2, stderr: "golaw: sqlc v1.31.1 not found\n"}, 1, []string{drift}, false},
		{"red an execution error", result{err: ErrAcceptance}, 0, []string{m7CheckClean}, false},
	}
	for _, row := range rows {
		if err := m7JudgeCheck(row.got, row.code, row.want); (err == nil) != row.ok {
			t.Fatalf("%s: %v", row.name, err)
		}
	}
}

func TestM7CheckCasesAreInTheCatalog(t *testing.T) {
	t.Parallel()
	want := []string{"clean-after-fill", "drift-edited", "drift-missing", "drift-third-party", "open-hole"}
	for _, catalog := range m7Catalogs() {
		if catalog.category == m7CheckCategory {
			if got := m7Names(catalog.cases); !slices.Equal(got, want) {
				t.Fatalf("m7-check rows = %q", got)
			}
			for _, c := range catalog.cases {
				if !c.gen {
					t.Fatalf("%s runs golaw gen without the generators", c.name)
				}
			}
			return
		}
	}
	t.Fatal("m7Catalogs lacks m7-check")
}

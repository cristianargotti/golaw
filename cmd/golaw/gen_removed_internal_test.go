package main

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/apply"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
)

func TestGenLinesNameEachRemovedEntry(t *testing.T) {
	t.Parallel()
	res := gen.Result{Removed: []manifest.Entry{{Kind: "adapter-sql", Name: "postgres.Plans"}}}
	report := apply.Report{Written: []string{"internal/adapters/out/postgres/zz_generated_plans.go"}}
	lines := genLines(res, report, nil)
	if !slices.Contains(lines, "removed adapter-sql postgres.Plans") {
		t.Errorf("lines = %q, want the removed entry named", lines)
	}
	if slices.Contains(genLines(gen.Result{}, apply.Report{}, nil), "removed adapter-sql postgres.Plans") {
		t.Error("a run that removed nothing named a removal")
	}
}

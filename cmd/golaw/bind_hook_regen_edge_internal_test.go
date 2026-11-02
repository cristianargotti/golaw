package main

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/imports"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestFixFileRunsTheStageAndBindsTheWriteToTheBytesRead(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const file = "label.go"
	writeHookRepoFile(t, root, file, "package label\n\nfunc label() string { return \"x\" }\n")
	stage := func(_ context.Context, content []byte) ([]byte, bool) {
		return []byte(strings.Replace(string(content), "\"x\"", "\"y\"", 1)), true
	}
	f, ok := fixFile(t.Context(), root, file, imports.Std(), stage)
	if !ok || !slices.Equal(f.parts, []string{partLintFmt}) || !strings.Contains(readRepoFile(t, root, file), "\"y\"") {
		t.Fatalf("fixed = %+v, %v", f, ok)
	}
	stale := landFixed(root, fixedFile{path: file, parts: []string{partFormat}}, []byte("package label\n"), []byte("other bytes"))
	if stale.skipped == "" || len(stale.parts) != 0 {
		t.Fatalf("a write over changed bytes was not skipped: %+v", stale)
	}
	if _, ok := fixFile(t.Context(), root, "absent.go", imports.Std(), nil); ok {
		t.Fatal("an unreadable file answered a fix")
	}
	gone := landFixed(t.TempDir()+"/missing", fixedFile{path: file}, []byte("x"), []byte("y"))
	if gone.skipped == "" {
		t.Fatalf("a root that cannot open was not named: %+v", gone)
	}
}

func TestRegenLinesNameTheSkippedFixAndTheImportReport(t *testing.T) {
	t.Parallel()
	o := regenOutcome{
		drift:   []gen.Drift{{Path: "a.go", Cause: gen.DriftDiffers}},
		fixed:   []fixedFile{{path: "b.go", skipped: "changed on disk", report: []string{"Plan names two packages"}}},
		outside: []string{"goverter"},
	}
	report := regenLines(model.DefaultLanguage, o)
	want := []string{"drift a.go " + gen.DriftDiffers + ": run golaw gen", "skipped b.go: changed on disk", "imports b.go: Plan names two packages"}
	if !slices.Equal(report.Lines, want) {
		t.Fatalf("lines = %q", report.Lines)
	}
	if !slices.Equal(report.Notices, []string{"run golaw gen: goverter regenerates converters outside the hook"}) ||
		report.Person[len(report.Person)-1] != "rode golaw gen: o goverter regenera os conversores fora do hook." {
		t.Fatalf("report = %+v", report)
	}
}

func TestRegenerateNamesAManifestItCannotRead(t *testing.T) {
	t.Parallel()
	files := gentest.Contracts()
	files[manifest.Path] = []byte("golaw-gen: 99\n")
	repo := gentest.Module(t, files)
	report := regenBinding(t).regenerate(t.Context(), hook.Repository{Root: repo.Root}, regenPolicy(), []string{"db/queries/plans.sql"})
	if len(report.Notices) != 1 || !strings.HasPrefix(report.Notices[0], "not regenerated in the hook: run golaw gen (") {
		t.Fatalf("report = %+v", report)
	}
}

func TestWiringMainReadsTheRootLayout(t *testing.T) {
	t.Parallel()
	m := manifest.Manifest{Entries: []manifest.Entry{{Kind: "port", Name: "growth.Ledger"}, {Kind: "wiring", Name: "main.Wire", Regenerable: []string{"zz_generated_wire.go"}}}}
	if got := wiringMain(m); got != "" {
		t.Fatalf("root layout = %q", got)
	}
	if got := wiringMain(manifest.Manifest{}); got != gen.MainByHand {
		t.Fatalf("no wiring = %q", got)
	}
}

package model_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

// plainSnapshot is a snapshot that folds nothing over a base.
type plainSnapshot struct{}

func (plainSnapshot) Files() []model.File          { return nil }
func (plainSnapshot) Read(string) ([]byte, error)  { return nil, model.ErrNotInSnapshot }
func (plainSnapshot) Class(string) model.FileClass { return model.ClassUnknown }

// overlaid is a snapshot that reads a base apart, as a hook overlay does.
type overlaid struct {
	plainSnapshot

	base model.Snapshot
}

func (o overlaid) Base() model.Snapshot { return o.base }

func TestCoreDefaultsHoldTheParitySurfaceFloor(t *testing.T) {
	t.Parallel()
	got := model.CoreDefaults().Surface
	if got.Rules != model.SurfaceParity {
		t.Errorf("Surface.Rules = %q, want %q", got.Rules, model.SurfaceParity)
	}
	if got.Analyzer.From != model.FromAuto {
		t.Errorf("Surface.Analyzer.From = %q, want %q", got.Analyzer.From, model.FromAuto)
	}
	if got.Analyzer.CoverageReport != model.DefaultCoverageReport || model.DefaultCoverageReport != "profile.cov" {
		t.Errorf("Surface.Analyzer.CoverageReport = %q, want profile.cov", got.Analyzer.CoverageReport)
	}
	if want := (model.CoverProfile{CoverMode: "set"}); got.Analyzer.Coverage != want {
		t.Errorf("Surface.Analyzer.Coverage = %+v, want %+v", got.Analyzer.Coverage, want)
	}
	lists := map[string][]string{
		"sources": got.Analyzer.Sources, "tests": got.Analyzer.Tests, "test_inclusions": got.Analyzer.TestInclusions,
		"exclusions": got.Analyzer.Exclusions, "coverage_exclusions": got.Analyzer.CoverageExclusions,
	}
	for name, list := range lists {
		if len(list) != 0 {
			t.Errorf("Surface.Analyzer.%s = %v, want empty: the whole tree is main code", name, list)
		}
	}
	if got.Globals == nil || len(got.Globals) != 0 {
		t.Errorf("Surface.Globals = %#v, want an empty list", got.Globals)
	}
	if model.DefaultCoverageNew != 80 {
		t.Errorf("DefaultCoverageNew = %d, want the Sonar way 80", model.DefaultCoverageNew)
	}
}

func TestSurfaceConstantsAreTheGolawYmlWords(t *testing.T) {
	t.Parallel()
	words := map[string]string{
		model.SurfaceParity: "parity", model.SurfaceStrict: "strict",
		model.FromAuto: "auto", model.FromProfile: "profile",
		model.FromProperties: "properties", model.FromPolicy: "policy",
	}
	for got, want := range words {
		if got != want {
			t.Errorf("constant %q, want %q", got, want)
		}
	}
}

func TestAnOverlayIsBasedAndAPlainSnapshotIsNot(t *testing.T) {
	t.Parallel()
	base := plainSnapshot{}
	tests := map[string]struct {
		snapshot model.Snapshot
		want     bool
	}{
		"an overlay over a base":       {snapshot: overlaid{base: base}, want: true},
		"a plain snapshot (near miss)": {snapshot: plainSnapshot{}, want: false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			based, ok := tc.snapshot.(model.Based)
			if ok != tc.want {
				t.Fatalf("snapshot is Based = %v, want %v", ok, tc.want)
			}
			if ok && based.Base() != model.Snapshot(base) {
				t.Errorf("Base() = %#v, want the base", based.Base())
			}
		})
	}
}

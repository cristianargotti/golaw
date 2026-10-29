package surface_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/surface"
)

func TestScopeOfReadsThePropertiesThroughTheSnapshot(t *testing.T) {
	t.Parallel()
	s := snapshotOf(map[string]string{"sonar-project.properties": pipelineProperties, "cmd/hub/main.go": "package main\n"})
	got, err := surface.ScopeOf(t.Context(), s, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != model.FromProperties || !slices.Equal(got.Exclusions, pipelineExclusions) {
		t.Fatalf("ScopeOf = %+v, want the properties' exclusions", got)
	}
	if reads := s.read(); !slices.Equal(reads, []string{surface.PropertiesFile}) {
		t.Errorf("reads = %v, want only %s", reads, surface.PropertiesFile)
	}
}

func TestScopeOfTakesTheProfileThePolicyCarries(t *testing.T) {
	t.Parallel()
	p := model.CoreDefaults()
	p.ProfileData = &model.ProfileData{Name: "surface", Analyzer: &model.AnalyzerPolicy{Exclusions: pipelineExclusions}}
	got, err := surface.ScopeOf(t.Context(), snapshotOf(map[string]string{"sonar-project.properties": "sonar.exclusions=x/**\n"}), &p)
	if err != nil || got.Source != model.FromProfile || got.Origin != "surface/analyzer.yml" {
		t.Fatalf("ScopeOf = %+v, %v, want the profile scope", got, err)
	}
	bundle := model.CoreDefaults()
	bundle.ProfileData = &model.ProfileData{Name: "fixture"}
	plain, err := surface.ScopeOf(t.Context(), snapshotOf(nil), &bundle)
	if err != nil || plain.Source != surface.SourceNone {
		t.Fatalf("a bundle without analyzer.yml = %+v, %v, want none", plain, err)
	}
	pinned := model.CoreDefaults()
	pinned.Surface.Analyzer.From = model.FromProperties
	if _, err := surface.ScopeOf(t.Context(), snapshotOf(nil), &pinned); !errors.Is(err, surface.ErrScopeSource) {
		t.Fatalf("a pinned absent file = %v, want ErrScopeSource", err)
	}
}

func TestScopeOfReportsAnUnreadablePropertiesFile(t *testing.T) {
	t.Parallel()
	s := snapshotOf(map[string]string{"sonar-project.properties": ""})
	s.failing["sonar-project.properties"] = model.ErrUnreadable
	if _, err := surface.ScopeOf(t.Context(), s, nil); !errors.Is(err, model.ErrUnreadable) {
		t.Fatalf("ScopeOf = %v, want the read error", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := surface.ScopeOf(ctx, snapshotOf(nil), nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("ScopeOf after the context ended = %v", err)
	}
}

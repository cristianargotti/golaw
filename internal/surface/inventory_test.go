package surface_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/surface"
)

func TestTakeReadsOnlyWhatTheNameLeavesOpen(t *testing.T) {
	t.Parallel()
	s := snapshotOf(map[string]string{
		"Dockerfile":         "FROM scratch\n",
		"Makefile":           "all:\n",
		"README.md":          "# hub\n",
		"cmd/hub/main.go":    "package main\n",
		"infra/10-base.yml":  template,
		"scripts/run":        "#!/bin/sh\n",
		"views/app.min.js":   "a()",
		"views/api.js":       "// generated\n",
		"views/logo.js":      "binary",
		"views/panel.html":   "<script>\nrun();\n</script>\n",
		"deploy/compose.yml": "services: {}\n",
	})
	s.classes["views/logo.js"] = model.ClassBinary
	s.classes["views/api.js"] = model.ClassGenerated
	inv, err := surface.Take(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Makefile", "deploy/compose.yml", "infra/10-base.yml", "scripts/run"}; !slices.Equal(s.read(), want) {
		t.Errorf("reads = %v, want %v: a name-settled file is never read", s.read(), want)
	}
	want := []surface.FileInfo{
		{Path: "Dockerfile", Language: surface.Dockerfile, Class: surface.ClassSource},
		{Path: "deploy/compose.yml", Language: surface.YAML, Class: surface.ClassSource},
		{Path: "infra/10-base.yml", Language: surface.CloudFormation, Class: surface.ClassSource},
		{Path: "scripts/run", Language: surface.Shell, Class: surface.ClassSource},
		{Path: "views/api.js", Language: surface.JavaScript, Class: surface.ClassGenerated},
		{Path: "views/app.min.js", Language: surface.JavaScript, Class: surface.ClassMinified},
		{Path: "views/panel.html", Language: surface.HTML, Class: surface.ClassSource},
	}
	if !slices.EqualFunc(inv.Files, want, sameInfo) {
		t.Errorf("Files = %+v, want %+v", inv.Files, want)
	}
	languages := []surface.Language{surface.HTML, surface.Dockerfile, surface.CloudFormation, surface.Shell, surface.YAML}
	if !slices.Equal(inv.Languages, languages) || len(inv.Unmeasured) != 0 {
		t.Errorf("Languages = %v, Unmeasured = %v, want %v and none: generated and minified scripts need no tool", inv.Languages, inv.Unmeasured, languages)
	}
}

// sameInfo compares two entries without their regions, which Take never
// reads for a name-settled file.
func sameInfo(a, b surface.FileInfo) bool {
	return a.Path == b.Path && a.Language == b.Language && a.Class == b.Class && a.Regions == nil
}

// TestTakeSettlesAFilePastTheReadBoundByItsName is a lock file, a values
// file and a blob larger than the read bound: none can be a CloudFormation
// template, so the name settles each and none is Unmeasured.
func TestTakeSettlesAFilePastTheReadBoundByItsName(t *testing.T) {
	t.Parallel()
	paths := []string{"bin/blob", "deploy/values.yaml", "package-lock.json"}
	s := snapshotOf(map[string]string{paths[0]: "", paths[1]: "", paths[2]: ""})
	for _, p := range paths {
		s.failing[p] = model.ErrTooLarge
	}
	inv, err := surface.Take(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Unmeasured) != 0 {
		t.Fatalf("Unmeasured = %q, want none: a file past the read bound is no template", inv.Unmeasured)
	}
	if len(inv.Files) != 1 || inv.Files[0].Path != "deploy/values.yaml" || inv.Files[0].Language != surface.YAML {
		t.Fatalf("Files = %+v, want the yaml document alone", inv.Files)
	}
}

func TestTakeNamesTheFilesItCannotJudge(t *testing.T) {
	t.Parallel()
	s := snapshotOf(map[string]string{
		"infra/big.yml":     "",
		"infra/broken.yml":  "AWSTemplateFormatVersion: [\n",
		"infra/broken.json": `{"Resources": {"B": {"Type": "AWS::S3::Bucket"`,
	})
	s.failing["infra/big.yml"] = model.ErrUnreadable
	inv, err := surface.Take(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Unmeasured) != 3 {
		t.Fatalf("Unmeasured = %q, want three causes", inv.Unmeasured)
	}
	if want := "infra/big.yml: " + model.ErrUnreadable.Error(); inv.Unmeasured[0] != want {
		t.Errorf("Unmeasured[0] = %q, want %q", inv.Unmeasured[0], want)
	}
	for _, cause := range inv.Unmeasured[1:] {
		if !slices.ContainsFunc([]string{"infra/broken.json: ", "infra/broken.yml: "}, func(prefix string) bool {
			return len(cause) > len(prefix) && cause[:len(prefix)] == prefix
		}) {
			t.Errorf("cause %q names no broken template", cause)
		}
	}
	if !slices.Equal(inv.Languages, []surface.Language{surface.YAML}) || len(inv.Of(surface.YAML)) != 1 {
		t.Errorf("Languages = %v: the broken yaml template is still read as yaml", inv.Languages)
	}
}

func TestTakeStopsWhenTheContextEnds(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := surface.Take(ctx, snapshotOf(map[string]string{"views/a.js": "a()\n"})); !errors.Is(err, context.Canceled) {
		t.Fatalf("Take = %v, want context.Canceled", err)
	}
	if inv, err := surface.Take(ctx, snapshotOf(nil)); err != nil || inv.Files != nil {
		t.Fatalf("an empty snapshot = %+v, %v, want nothing and no error", inv, err)
	}
}

func TestALanguageCountsThroughTheFilesAJudgeReads(t *testing.T) {
	t.Parallel()
	inv, err := surface.Take(t.Context(), snapshotOf(map[string]string{
		"internal/panel/testdata/growth.jsx": "export const a = 1;\n",
		"views/charts.min.css":               "a{}",
		"web/types.d.ts":                     "export type A = string;\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(inv.Languages, []surface.Language{surface.TypeScript}) || len(inv.Files) != 3 {
		t.Fatalf("Languages = %v, Files = %+v: a fixture or a minified file names no language, a declaration does", inv.Languages, inv.Files)
	}
}

func TestHasAndOfAnswerOneLanguage(t *testing.T) {
	t.Parallel()
	inv, err := surface.Take(t.Context(), snapshotOf(map[string]string{
		"views/a.js": "a()\n", "views/b.js": "b()\n", "deploy/install.py": "print(1)\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !inv.Has(surface.JavaScript) || !inv.Has(surface.Python) || inv.Has(surface.CSS) {
		t.Errorf("Has: languages = %v", inv.Languages)
	}
	if got := inv.Of(surface.JavaScript); len(got) != 2 || got[0].Path != "views/a.js" || got[1].Path != "views/b.js" {
		t.Errorf("Of(javascript) = %+v", got)
	}
	if got := inv.Of(surface.CSS); got != nil {
		t.Errorf("Of(css) = %+v, want none", got)
	}
}

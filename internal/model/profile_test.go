package model_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

// fullProfileData carries every section with one neutral value.
func fullProfileData() *model.ProfileData {
	equals := "true"
	return &model.ProfileData{
		Name: "fixture", Version: "1.2.3", Digest: "sha256:" + strings.Repeat("a", 64),
		Terms:      []string{"legacyci"},
		Paths:      []string{"AGENT-NOTES.md", ".agentconfig/"},
		Layout:     &model.Layout{Main: model.DefaultMain, Decisions: model.DefaultDecisions},
		Templates:  model.TemplateRules{Exclude: []string{".github/"}},
		Targets:    []string{"hooks-test"},
		Dockerfile: &model.DockerfileRules{Stages: []string{"base", "builder"}, FinalImage: "scratch"},
		Registry:   &model.RegistryRule{Pattern: `^registry\.example\.test/`},
		Assertions: []model.Assertion{{
			Path: "deploy/values.yaml", Key: []string{"securityContext", "runAsNonRoot"},
			Kind: model.AssertBool, Equals: &equals, Missing: model.MissingFinding, Files: model.FilesRequired,
		}},
	}
}

func TestProfileDataSections(t *testing.T) {
	t.Parallel()
	var none *model.ProfileData
	if none.Section(model.SectionTerms) {
		t.Error("a nil ProfileData carries no section")
	}
	full, empty := fullProfileData(), &model.ProfileData{Name: "fixture"}
	for _, name := range allSections() {
		if !full.Section(name) {
			t.Errorf("the full data must carry %s", name)
		}
		if empty.Section(name) {
			t.Errorf("data with every section empty must not carry %s", name)
		}
	}
	if full.Section("branch") {
		t.Error("an unknown section is never carried")
	}
}

// allSections lists every section name the data answers by name.
func allSections() []string {
	return []string{
		model.SectionAssertions, model.SectionDockerfile, model.SectionLayout, model.SectionPaths,
		model.SectionRegistry, model.SectionTargets, model.SectionTemplates, model.SectionTerms,
	}
}

func TestProfileDataConstants(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]string{
		model.AssertBool: "bool", model.AssertInt: "int", model.AssertString: "string",
		model.MissingFinding: "finding", model.MissingIgnore: "ignore",
		model.FilesRequired: "required", model.FilesOptional: "optional",
	} {
		if name != want {
			t.Errorf("constant %q, want %q", name, want)
		}
	}
}

func TestEffectivePolicyStartsWithoutProfileData(t *testing.T) {
	t.Parallel()
	p := model.CoreDefaults()
	if p.ProfileData != nil {
		t.Errorf("CoreDefaults carries profile data: %+v", p.ProfileData)
	}
	p.ProfileData = &model.ProfileData{Name: "fixture", Terms: []string{"legacyci"}}
	if !p.ProfileData.Section(model.SectionTerms) || p.ProfileData.Section(model.SectionPaths) {
		t.Error("the field carries the data as given")
	}
}

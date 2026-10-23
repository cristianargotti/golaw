package profile_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/profile"
)

// fixtureRules is the data of rules.yml of the fixture bundle, in one flow form.
const fixtureRules = `golaw_profile: 1
terms: [legacyci]
paths: ["AGENT-NOTES.md", ".agentconfig/"]
layout: {main: root, decisions: docs/adr}
templates: {exclude: [".github/"]}
targets: [hooks-test, image-lint]
dockerfile:
  stages: [certs, base, ci, builder]
  final_image: scratch
  installs:
    - {stage: ci, packages: [ca-certificates, git, tzdata]}
  pinned_installs: true
  ignore_comments: ["# lint ignore"]
registry: {pattern: "^registry\\.example\\.test/"}
assertions:
  - {path: deploy/values.yaml, key: [securityContext, runAsNonRoot], kind: bool, equals: "true", missing: finding, files: required}
  - {path: deploy/values.yaml, key: [podAnnotations, example.test/do-not-disrupt], kind: string, equals: "true", missing: finding, files: required}
  - {path: "deploy/jobs/*.yaml", key: [spec, activeDeadlineSeconds], kind: int, missing: finding, files: optional}
`

func TestDecodeReadsEverySection(t *testing.T) {
	t.Parallel()
	d, err := profile.Decode([]byte(fixtureRules))
	if err != nil {
		t.Fatal(err)
	}
	switch {
	case d.Name != "" || d.Version != "" || d.Digest != "":
		t.Errorf("Decode fills no identity: %+v", d)
	case !slices.Equal(d.Terms, []string{"legacyci"}), !slices.Equal(d.Paths, []string{"AGENT-NOTES.md", ".agentconfig/"}):
		t.Errorf("terms and paths = %v, %v", d.Terms, d.Paths)
	case d.Layout == nil || *d.Layout != (model.Layout{Main: "root", Decisions: "docs/adr"}):
		t.Errorf("layout = %+v", d.Layout)
	case !slices.Equal(d.Templates.Exclude, []string{".github/"}), !slices.Equal(d.Targets, []string{"hooks-test", "image-lint"}):
		t.Errorf("templates and targets = %+v, %v", d.Templates, d.Targets)
	case d.Dockerfile == nil || !slices.Equal(d.Dockerfile.Stages, []string{"certs", "base", "ci", "builder"}) ||
		d.Dockerfile.FinalImage != "scratch" || !d.Dockerfile.PinnedInstalls || !slices.Equal(d.Dockerfile.IgnoreComments, []string{"# lint ignore"}):
		t.Errorf("dockerfile = %+v", d.Dockerfile)
	case len(d.Dockerfile.Installs) != 1 || d.Dockerfile.Installs[0].Stage != "ci" || !slices.Equal(d.Dockerfile.Installs[0].Packages, []string{"ca-certificates", "git", "tzdata"}):
		t.Errorf("installs = %+v", d.Dockerfile.Installs)
	case d.Registry == nil || d.Registry.Pattern != `^registry\.example\.test/`:
		t.Errorf("registry = %+v", d.Registry)
	case len(d.Assertions) != 3:
		t.Errorf("assertions = %+v", d.Assertions)
	}
	first, last := d.Assertions[0], d.Assertions[2]
	if first.Path != "deploy/values.yaml" || !slices.Equal(first.Key, []string{"securityContext", "runAsNonRoot"}) ||
		first.Kind != model.AssertBool || first.Equals == nil || *first.Equals != "true" || first.Missing != model.MissingFinding || first.Files != model.FilesRequired {
		t.Errorf("first assertion = %+v", first)
	}
	if last.Equals != nil || last.Kind != model.AssertInt || last.Files != model.FilesOptional || last.Path != "deploy/jobs/*.yaml" {
		t.Errorf("last assertion = %+v", last)
	}
	for _, name := range allSections() {
		if !d.Section(name) {
			t.Errorf("the fixture carries %s", name)
		}
	}
}

func TestDecodeIsStrict(t *testing.T) {
	t.Parallel()
	for name, text := range map[string]string{
		"unknown key":    fixtureRules + "extra: 1\n",
		"unknown nested": strings.Replace(fixtureRules, "final_image: scratch", "final_image: scratch\n  flavor: x", 1),
		"duplicate key":  fixtureRules + "terms: [again]\n",
		"other schema":   strings.Replace(fixtureRules, "golaw_profile: 1", "golaw_profile: 2", 1),
		"no schema":      strings.Replace(fixtureRules, "golaw_profile: 1\n", "", 1),
		"second doc":     fixtureRules + "---\nterms: []\n",
		"wrong type":     strings.Replace(fixtureRules, "terms: [legacyci]", "terms: legacyci", 1),
		"empty":          "",
		"branch":         fixtureRules + "branch: \"^feature/\"\n",
		"not yaml":       "terms: [\n",
	} {
		_, err := profile.Decode([]byte(text))
		if !errors.Is(err, profile.ErrBundle) || !errors.Is(err, model.ErrPolicy) {
			t.Errorf("%s: %v, want ErrBundle under ErrPolicy", name, err)
		}
		if name == "branch" && !strings.Contains(err.Error(), "branch is not supported in this version") {
			t.Errorf("the reserved key names its state: %v", err)
		}
		if strings.Contains(err.Error(), "\n") {
			t.Errorf("%s: the refusal spans lines, and a golaw: line is one: %q", name, err)
		}
		if name == "unknown key" && !strings.Contains(err.Error(), "field extra not found") {
			t.Errorf("the refusal names the unknown key: %v", err)
		}
		newer := "a bundle written for a newer golaw needs that golaw"
		wants := name == "unknown key" || name == "unknown nested" || name == "other schema"
		if strings.Contains(err.Error(), newer) != wants {
			t.Errorf("%s: the remedy of a newer bundle is named %v: %v", name, wants, err)
		}
	}
}

func TestDecodeAcceptsAnEmptyProfile(t *testing.T) {
	t.Parallel()
	d, err := profile.Decode([]byte("golaw_profile: 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range allSections() {
		if d.Section(name) {
			t.Errorf("an empty profile carries no %s", name)
		}
	}
	if err := profile.Validate(d); err != nil {
		t.Errorf("an empty profile is valid: %v", err)
	}
}

// allSections lists every section name the data answers by name.
func allSections() []string {
	return []string{
		model.SectionAssertions, model.SectionDockerfile, model.SectionLayout, model.SectionPaths,
		model.SectionRegistry, model.SectionTargets, model.SectionTemplates, model.SectionTerms,
	}
}

package bootstrap_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
)

// excludedPlan is the plan of a root whose tree holds one kit file with
// other bytes, one with the rendered bytes and one with recorded local
// edits under .github/, and kit files beside it, a sibling directory whose
// name starts like the prefix among them.
func excludedPlan(t *testing.T) (bootstrap.TemplatePlan, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".github"), 0o750); err != nil {
		t.Fatal(err)
	}
	recorded := install.Digest([]byte("recorded\n"))
	writeBefore(t, filepath.Join(root, bootstrap.PolicyFile), fmt.Sprintf(
		"golaw: 1\nkit: {version: 0.1.0}\nmanaged:\n  - {path: .github/edited.yml, sha256: %s}\n", recorded))
	writeBefore(t, filepath.Join(root, ".github", "dependabot.yml"), "theirs\n")
	writeBefore(t, filepath.Join(root, ".github", "same.yml"), "same\n")
	writeBefore(t, filepath.Join(root, ".github", "edited.yml"), "edited\n")
	plan, err := bootstrap.PlanTemplates(root, []assets.Rendered{
		templateFile(".github/new.yml", "new\n", assets.OwnedByKit),
		templateFile(".github/dependabot.yml", "ours\n", assets.OwnedByKit),
		templateFile(".github/same.yml", "same\n", assets.OwnedByKit),
		templateFile(".github/edited.yml", "kit\n", assets.OwnedByKit),
		templateFile(".githubx/keep.yml", "keep\n", assets.OwnedByKit),
		templateFile("golaw.mk", "ci:\n", assets.OwnedByKit),
	})
	if err != nil {
		t.Fatal(err)
	}
	plan.Ownership.Protected = []string{".github/owned.yml", bootstrap.PolicyFile}
	plan.Irregular = []string{".github/link.yml", "docs/link.md"}
	return plan, root
}

func changePaths(changes []install.Change) []string {
	out := make([]string, 0, len(changes))
	for _, c := range changes {
		out = append(out, c.Path)
	}
	return out
}

func TestExcludeLeavesTheDestinationsOfAPrefixToTheirOwner(t *testing.T) {
	t.Parallel()
	plan, root := excludedPlan(t)
	if len(plan.LocalEdits) != 1 || !slices.Contains(changePaths(plan.Proposed), ".github/dependabot.yml") {
		t.Fatalf("the plan before the exclusion = %+v", plan)
	}
	plan.Exclude([]string{".github/"})
	plan.Exclude([]string{".github/"})
	want := []string{".github/dependabot.yml", ".github/edited.yml", ".github/link.yml", ".github/new.yml", ".github/owned.yml", ".github/same.yml"}
	if !slices.Equal(plan.Excluded, want) {
		t.Errorf("excluded = %v", plan.Excluded)
	}
	if got := changePaths(plan.Changes); !slices.Equal(got, []string{".githubx/keep.yml", "golaw.mk"}) || len(plan.Proposed) != 0 || len(plan.LocalEdits) != 0 {
		t.Errorf("changes = %v, proposed = %v, local edits = %+v", got, changePaths(plan.Proposed), plan.LocalEdits)
	}
	if slices.ContainsFunc(plan.Managed, func(m policy.ManagedFile) bool { return strings.HasPrefix(m.Path, ".github/") }) || len(plan.Managed) != 2 {
		t.Errorf("managed = %+v", plan.Managed)
	}
	if !slices.Equal(plan.Ownership.Protected, []string{bootstrap.PolicyFile}) || !slices.Equal(plan.Irregular, []string{"docs/link.md"}) {
		t.Errorf("protected = %v, irregular = %v", plan.Ownership.Protected, plan.Irregular)
	}
	if err := bootstrap.ApplyTemplates(root, plan, io.Discard); err != nil {
		t.Fatal(err)
	}
	requireTemplateAbsent(t, root, ".github/new.yml")
	if _, err := os.Stat(filepath.Join(root, ".githubx", "keep.yml")); err != nil {
		t.Errorf("a destination beside the prefix was not written: %v", err)
	}
}

func TestExcludeOfOneFileAndOfNothing(t *testing.T) {
	t.Parallel()
	plan, _ := excludedPlan(t)
	plan.Exclude([]string{"golaw.mk", bootstrap.PolicyFile, ".github"})
	if !slices.Equal(plan.Excluded, []string{"golaw.mk"}) || !slices.Contains(plan.Ownership.Protected, bootstrap.PolicyFile) {
		t.Errorf("excluded = %v, protected = %v", plan.Excluded, plan.Ownership.Protected)
	}
	if !slices.Contains(changePaths(plan.Changes), ".github/new.yml") {
		t.Errorf("a prefix without its slash covered a tree: %v", changePaths(plan.Changes))
	}
	untouched, _ := excludedPlan(t)
	before := len(untouched.Changes)
	untouched.Exclude(nil)
	if len(untouched.Changes) != before || untouched.Excluded != nil {
		t.Errorf("no prefix changed the plan: %+v", untouched)
	}
}

func TestTemplateDiffNamesWhatTheProfileLeavesOut(t *testing.T) {
	t.Parallel()
	for language, want := range map[string]string{
		"pt-BR": "Deixados para a plataforma: .github/dependabot.yml, .github/edited.yml, .github/link.yml, .github/new.yml, .github/owned.yml, .github/same.yml.\n",
		"en":    "Left to the platform: .github/dependabot.yml, .github/edited.yml, .github/link.yml, .github/new.yml, .github/owned.yml, .github/same.yml.\n",
	} {
		plan, _ := excludedPlan(t)
		plan.Language = language
		plan.Exclude([]string{".github/"})
		var out bytes.Buffer
		if err := bootstrap.WriteTemplateDiff(&out, plan); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), want) || strings.Contains(out.String(), "+++ .github/") {
			t.Errorf("%s diff:\n%s", language, out.String())
		}
	}
	plain, _ := excludedPlan(t)
	var out bytes.Buffer
	if err := bootstrap.WriteTemplateDiff(&out, plain); err != nil || strings.Contains(out.String(), "Deixados") {
		t.Errorf("a plan without an exclusion names one: %v\n%s", err, out.String())
	}
}

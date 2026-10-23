package paths_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/paths"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/profiletest"
	"github.com/cristianargotti/golaw/internal/proof"
)

// profiled runs paths.profile-forbidden in a mode over the named targets
// under the proof policy with a profile's data; the tree holds a target
// only in a scan, as a hook's candidate does not exist yet.
func profiled(t *testing.T, data *model.ProfileData, mode model.Mode, targets ...string) model.Result {
	t.Helper()
	files := make(map[string][]byte, len(targets))
	for _, target := range targets {
		files[target] = []byte("x\n")
	}
	tree := proof.NewTree(files)
	in := model.Input{Mode: mode, Policy: proof.Policy(), Snapshot: tree, Targets: tree.Files()}
	if mode == model.ModeHook {
		in.Snapshot = proof.NewTree(nil)
	}
	if data != nil {
		in.Policy.Profile, in.Policy.ProfileData = data.Name, data
	}
	res, err := paths.ProfileForbidden().Run(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// TestProfileForbidden pins paths.profile-forbidden (contract D5): a glob
// of the profile matches the whole slash path, a glob that ends in a slash
// forbids its directory tree, and a file under testdata is never judged.
func TestProfileForbidden(t *testing.T) {
	t.Parallel()
	res := profiled(t, profiletest.Data(t), model.ModeScan,
		"AGENT-NOTES.md", ".agentconfig/settings.json", ".agentconfig/deep/x.txt",
		"docs/AGENT-NOTES.md", "testdata/AGENT-NOTES.md", ".agentconfigx/a.json", "AGENT-NOTES.md.bak", ".agentconfig",
	)
	want := []string{
		".agentconfig/deep/x.txt profile-forbidden-path", ".agentconfig/settings.json profile-forbidden-path",
		"AGENT-NOTES.md profile-forbidden-path",
	}
	if got := slices.Sorted(slices.Values(pathsOf(res))); res.Status.Kind != model.StatusMeasured || !slices.Equal(got, want) {
		t.Fatalf("findings = %v (%+v), want %v", got, res.Status, want)
	}
	if !strings.Contains(res.Findings[0].Message, "the profile forbids") || res.Scope != "8 paths" {
		t.Errorf("finding = %+v, scope %q", res.Findings[0], res.Scope)
	}
}

// TestProfileForbiddenReadsTheGlobGrammar pins the path.Match grammar of a
// glob: a star stays inside one segment, in a file glob and in a directory.
func TestProfileForbiddenReadsTheGlobGrammar(t *testing.T) {
	t.Parallel()
	data := *profiletest.Data(t)
	data.Paths = []string{"*.local.md", "config/*/secrets/"}
	res := profiled(t, &data, model.ModeStaged, "notes.local.md", "docs/a.local.md", "config/dev/secrets/a.txt", "config/secrets/a.txt")
	want := []string{"config/dev/secrets/a.txt profile-forbidden-path", "notes.local.md profile-forbidden-path"}
	if got := slices.Sorted(slices.Values(pathsOf(res))); !slices.Equal(got, want) {
		t.Errorf("findings = %v, want %v", got, want)
	}
}

// TestProfileForbiddenFoldsCase pins A14: a name a case-insensitive
// filesystem aliases to a forbidden one is forbidden in any case, and the
// same name elsewhere or with a suffix stays free.
func TestProfileForbiddenFoldsCase(t *testing.T) {
	t.Parallel()
	data := *profiletest.Data(t)
	data.Paths = []string{"AGENTS.md", ".Agent/"}
	res := profiled(t, &data, model.ModeScan, "agents.md", ".agent/x.json", "docs/agents.md", "AGENTS.md.bak")
	want := []string{".agent/x.json profile-forbidden-path", "agents.md profile-forbidden-path"}
	if got := slices.Sorted(slices.Values(pathsOf(res))); !slices.Equal(got, want) {
		t.Errorf("findings = %v, want %v", got, want)
	}
}

// TestProfileForbiddenDeniesAWriteBeforeTheFileExists pins the file scope:
// an agent's candidate is judged by its name in the hook.
func TestProfileForbiddenDeniesAWriteBeforeTheFileExists(t *testing.T) {
	t.Parallel()
	res := profiled(t, profiletest.Data(t), model.ModeHook, ".agentconfig/new.json")
	if got := pathsOf(res); !slices.Equal(got, []string{".agentconfig/new.json profile-forbidden-path"}) {
		t.Errorf("candidate findings = %v", got)
	}
}

// TestProfileForbiddenAppliesOnlyWithItsSection pins the gates of D5 and
// the place of the check in the family.
func TestProfileForbiddenAppliesOnlyWithItsSection(t *testing.T) {
	t.Parallel()
	if res := profiled(t, nil, model.ModeScan, "AGENT-NOTES.md"); res.Status.Kind != model.StatusNotApplicable ||
		!strings.Contains(res.Status.Reason, "no profile is enrolled") {
		t.Errorf("core only = %+v", res)
	}
	noPaths := *profiletest.Data(t)
	noPaths.Paths = nil
	if res := profiled(t, &noPaths, model.ModeScan, "AGENT-NOTES.md"); res.Status.Kind != model.StatusNotApplicable ||
		res.Status.Reason != "the profile carries no paths" {
		t.Errorf("no paths section = %+v", res)
	}
	ids := make([]string, 0, len(paths.Checks()))
	for _, c := range paths.Checks() {
		ids = append(ids, c.Spec().ID)
	}
	if want := []string{"paths.forbidden", "paths.generated-tracked", "paths.secret-material", "paths.profile-forbidden"}; !slices.Equal(ids, want) {
		t.Errorf("Checks() = %v, want %v", ids, want)
	}
	if s := paths.ProfileForbidden().Spec(); s.Scope != model.ScopeFile || s.Rule != "rules/4.2@1" || len(s.Needs) != 0 {
		t.Errorf("spec = %+v", s)
	}
}

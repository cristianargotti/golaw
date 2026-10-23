package main

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// TestInitAsksToEnrollAMovedPinAfterItsCommit walks the pin move the guides
// teach over a committed policy: init --profile --apply writes the pin and
// names it in the line that asks for the commit and a second init, which
// then enrolls the committed pin, so the scan judges under the profile.
func TestInitAsksToEnrollAMovedPinAfterItsCommit(t *testing.T) {
	t.Parallel()
	ta := boundApp(t)
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	profiletest.Install(t, dirs.Config)
	repo := adopted(t, ta, true)
	before := currentPolicy(t, repo.Root)
	applied := capabilityRun(t, ta, exitClean, "init", "--apply", "--profile", profiletest.Name, repo.Root)
	if want := enrollAfterCommitLine(model.DefaultLanguage, profiletest.Name) + "\n"; !strings.HasSuffix(applied, want) {
		t.Fatalf("applied = %q, want the suffix %q", applied, want)
	}
	if got := enrolledDigest(t, ta, repo.Root); got != digestOf(before) {
		t.Fatalf("the record moved before the commit: %s", got)
	}
	repo.Git("add", "-A")
	repo.Commit("build: pin the fixture profile", nil)
	if out := capabilityRun(t, ta, exitClean, "init", "--apply", repo.Root); !strings.Contains(out, "enrolled: ") {
		t.Fatalf("second apply = %q", out)
	}
	// The enrolled pin is in force: the scan runs the profile checks.
	if out := capabilityRun(t, ta, exitFindings, "scan", repo.Root); !strings.Contains(out, "policy.profile-targets") {
		t.Fatalf("scan under the pin = %q", out)
	}
}

func TestEnrollAfterCommitLineNamesWhatChanged(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ language, profile, want string }{
		"a moved pin in pt-BR":        {model.DefaultLanguage, "fixture", "Faça o commit de golaw.yml e rode golaw init --apply outra vez para registrar o pin do perfil fixture."},
		"a moved pin in English":      {"en", "fixture", "Commit golaw.yml, then run golaw init --apply again to enroll the pin of profile fixture."},
		"a changed policy in pt-BR":   {model.DefaultLanguage, "", "Faça o commit de golaw.yml e rode golaw init --apply outra vez para registrar a política alterada."},
		"a changed policy in English": {"en", "", "Commit golaw.yml, then run golaw init --apply again to enroll the changed policy."},
	}
	for name, tc := range cases {
		if got := enrollAfterCommitLine(tc.language, tc.profile); got != tc.want {
			t.Errorf("%s: %q, want %q", name, got, tc.want)
		}
	}
}

// pinnedPolicy is the rendered policy of a Go module, pinned to ref when
// ref is not nil.
func pinnedPolicy(t *testing.T, ref *policy.ProfileRef) []byte {
	t.Helper()
	in, err := bootstrap.Inputs{
		KitVersion: testVersion, Language: model.DefaultLanguage, Profile: ref,
		Layout: bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/adr", Go: true},
	}.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	data, err := bootstrap.Render(in)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestMovedProfileNamesAMovedPinOnly(t *testing.T) {
	t.Parallel()
	ref := profiletest.Ref(t)
	newer := ref
	newer.Version, newer.Digest = "1.3.0", "sha256:"+strings.Repeat("b", 64)
	core, pinned, moved := pinnedPolicy(t, nil), pinnedPolicy(t, &ref), pinnedPolicy(t, &newer)
	cases := map[string]struct {
		current, data []byte
		want          string
	}{
		"a moved version":         {pinned, moved, profiletest.Name},
		"a first pin":             {core, pinned, profiletest.Name},
		"near the same pin":       {pinned, pinned, ""},
		"near no pin":             {core, core, ""},
		"near an unreadable plan": {pinned, []byte("golaw: ["), ""},
	}
	for name, tc := range cases {
		if got := movedProfile(tc.current, tc.data); got != tc.want {
			t.Errorf("%s: %q, want %q", name, got, tc.want)
		}
	}
}

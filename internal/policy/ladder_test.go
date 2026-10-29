package policy_test

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// ladderPolicy switches two rungs and commits two waivers out of order, one
// of them expired.
const ladderPolicy = `golaw: 1
kit: {version: 0.1.0}
ladder: {pre_push: false, test_staged: true}
vulnerability_waivers:
  - {id: GO-2026-7001, until: 2026-12-01, record: docs/adr/0012-b.md}
  - {id: GO-2026-6603, until: 2020-01-01, record: docs/adr/0011-a.md}
`

func TestCompileCarriesTheLadderAndEveryWaiver(t *testing.T) {
	t.Parallel()
	p, err := policy.Compile(decode(t, ladderPolicy), now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := (model.Ladder{PreCommit: true, CommitMsg: true, TestStaged: true, SurfaceLint: true, HookFeedback: true}); p.Ladder != want {
		t.Errorf("ladder = %+v, want %+v", p.Ladder, want)
	}
	want := []model.Waiver{
		{ID: "GO-2026-6603", Until: "2020-01-01", Record: "docs/adr/0011-a.md"},
		{ID: "GO-2026-7001", Until: "2026-12-01", Record: "docs/adr/0012-b.md"},
	}
	if !slices.Equal(p.Waivers, want) {
		t.Errorf("waivers = %+v, want %+v", p.Waivers, want)
	}
	minimal, err := policy.Compile(decode(t, minimalPolicy), now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if minimal.Ladder != model.DefaultLadder() || minimal.Waivers == nil || len(minimal.Waivers) != 0 {
		t.Errorf("minimal ladder %+v, waivers %+v", minimal.Ladder, minimal.Waivers)
	}
	if policy.Digest(decode(t, ladderPolicy)) != policy.Digest(decode(t, strings.Replace(ladderPolicy, "pre_push: false", "pre_push: true", 1))) {
		t.Error("the ladder stays out of the digest")
	}
}

func writePolicy(t *testing.T, root, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, policy.FileName), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCheckoutLadderReadsTheFileAsItStands(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, ok := policy.CheckoutLadder(root); ok {
		t.Error("a checkout without golaw.yml answers false")
	}
	writePolicy(t, root, "golaw: 1\nkit: {version: 0.1.0}\nladder: {pre_commit: false}\n")
	got, ok := policy.CheckoutLadder(root)
	if want := (model.Ladder{CommitMsg: true, PrePush: true, SurfaceLint: true, HookFeedback: true}); !ok || got != want {
		t.Errorf("ladder = %+v, %v, want %+v", got, ok, want)
	}
	writePolicy(t, root, "golaw: 1\nkit: {version: 0.1.0}\nladders: {}\n")
	if _, ok := policy.CheckoutLadder(root); ok {
		t.Error("a file that does not decode answers false")
	}
	writePolicy(t, root, "golaw: 1\n"+strings.Repeat("#", model.MaxPolicyBytes)+"\n")
	if _, ok := policy.CheckoutLadder(root); ok {
		t.Error("a file over the bound answers false")
	}
}

func TestCheckoutLadderNeverFollowsALink(t *testing.T) {
	t.Parallel()
	root, elsewhere := t.TempDir(), t.TempDir()
	writePolicy(t, elsewhere, "golaw: 1\nkit: {version: 0.1.0}\nladder: {pre_commit: false}\n")
	if err := os.Symlink(filepath.Join(elsewhere, policy.FileName), filepath.Join(root, policy.FileName)); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("creating a symbolic link needs a privilege this runner may lack")
		}
		t.Fatal(err)
	}
	if _, ok := policy.CheckoutLadder(root); ok {
		t.Error("a linked golaw.yml answers false")
	}
}

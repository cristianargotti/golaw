package rung

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/source"
)

// pinned is a policy that pins the kit version and writes the ladder block
// it is given.
func pinned(version, ladder string) string {
	return "golaw: 1\nkit: {version: " + version + "}\n" + ladder
}

// repository commits the files of a module and the policy, none when it is
// empty, and answers the repository.
func repository(t *testing.T, golaw string) *gittest.Repo {
	t.Helper()
	r := gittest.New(t)
	files := map[string]string{"go.mod": "module example.test/m\n\ngo 1.27\n"}
	if golaw != "" {
		files[policy.FileName] = golaw
	}
	r.Commit("build: start the module", files)
	return r
}

// loading is the load cmd binds, policy.LoadNext over a state directory of
// the test and a binary of the version, empty for a development build.
type loading struct {
	state, version string
	calls          int
}

func (l *loading) load(ctx context.Context, dir string) (*policy.Loaded, error) {
	l.calls++
	discover := func(ctx context.Context, dir string) (policy.Discovery, error) {
		repo, err := source.Discover(ctx, dir)
		if err != nil {
			return policy.Discovery{}, fmt.Errorf("discover: %w", err)
		}
		return policy.Discovery{Root: repo.Root, Head: repo.Head}, nil
	}
	in := policy.Input{
		Dir: dir, Dirs: policy.Dirs{State: l.state, Config: l.state}, Version: l.version,
		Now: time.Date(2026, time.October, 19, 12, 0, 0, 0, time.UTC), Discover: discover,
	}
	loaded, err := policy.LoadNext(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("load the policy of %s: %w", dir, err)
	}
	return loaded, nil
}

func gated(t *testing.T, r *gittest.Repo, version, name string) (*loading, armed, Outcome, bool) {
	t.Helper()
	l := &loading{state: t.TempDir(), version: version}
	a, out, ok := gate(t.Context(), Dependencies{Dir: r.Root, Load: l.load}, name)
	return l, a, out, ok
}

func TestNamesFollowTheOrderGitRunsThem(t *testing.T) {
	t.Parallel()
	if got := Names(); !slices.Equal(got, []string{"pre-commit", "commit-msg", "pre-push"}) {
		t.Errorf("Names() = %v", got)
	}
}

func TestGateSwitchesARungOffBeforeAnyOtherWork(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, key string }{{PreCommit, "pre_commit"}, {CommitMsg, "commit_msg"}, {PrePush, "pre_push"}} {
		r := repository(t, pinned("0.1.0", "ladder: {"+c.key+": false}\n"))
		l, _, out, ok := gated(t, r, "0.2.0", c.name)
		if ok || out.Code != 0 || len(out.Lines) != 0 || l.calls != 0 {
			t.Errorf("%s switched off: %+v, %v, %d loads", c.name, out, ok, l.calls)
		}
		for _, other := range slices.DeleteFunc(Names(), func(n string) bool { return n == c.name }) {
			if _, _, out, ok := gated(t, r, "0.2.0", other); ok || out.Code != 2 {
				t.Errorf("%s stays armed beside %s: %+v", other, c.name, out)
			}
		}
	}
}

func TestGateRefusesAPinInEitherDirection(t *testing.T) {
	t.Parallel()
	r := repository(t, pinned("0.2.0", ""))
	for _, version := range []string{"0.1.0", "0.3.0"} {
		_, _, out, ok := gated(t, r, version, PreCommit)
		want := "pins kit version 0.2.0 and this binary is " + version + ": install the pinned version or run init --update"
		if ok || out.Code != 2 || len(out.Lines) != 2 || !strings.HasPrefix(out.Lines[0], "golaw pre-commit: ") || !strings.Contains(out.Lines[0], want) {
			t.Errorf("binary %s: %+v", version, out)
			continue
		}
		if out.Lines[1] != "golaw recusou o hook pre-commit; a causa e a correção estão nas linhas acima." {
			t.Errorf("human line = %q", out.Lines[1])
		}
	}
	for _, version := range []string{"0.2.0", ""} {
		if _, a, out, ok := gated(t, r, version, PreCommit); !ok || out.Code != 0 || a.loaded.Policy.KitVersion != "0.2.0" || a.ladder != model.DefaultLadder() {
			t.Errorf("binary %q: %+v, %+v", version, a, out)
		}
	}
}

func TestGateRefusesAnEnrollmentMismatch(t *testing.T) {
	t.Parallel()
	r := repository(t, pinned("0.1.0", ""))
	l := &loading{state: t.TempDir()}
	record := policy.Record{Root: r.Root, PolicyDigest: strings.Repeat("0", 64), KitVersion: "0.1.0", WrittenBy: policy.WrittenByInit, WrittenAt: time.Now()}
	if err := policy.WriteRecord(l.state, record); err != nil {
		t.Fatal(err)
	}
	_, out, ok := gate(t.Context(), Dependencies{Dir: r.Root, Load: l.load}, CommitMsg)
	if ok || out.Code != 2 || !strings.Contains(out.Lines[0], "does not match the enrollment record") {
		t.Errorf("mismatch: %+v", out)
	}
}

func TestGateLetsAPositivelyUnenrolledLocationGo(t *testing.T) {
	t.Parallel()
	r := repository(t, "")
	l, _, out, ok := gated(t, r, "0.2.0", PrePush)
	if ok || out.Code != 0 || len(out.Lines) != 0 || l.calls != 1 {
		t.Errorf("unenrolled: %+v, %v, %d loads", out, ok, l.calls)
	}
}

func TestGateReadsThePolicyLadderWhenTheCheckoutHoldsNone(t *testing.T) {
	t.Parallel()
	r := repository(t, pinned("0.1.0", "ladder: {pre_push: false, test_staged: true}\n"))
	r.Remove(policy.FileName)
	if _, _, out, ok := gated(t, r, "", PrePush); ok || out.Code != 0 || len(out.Lines) != 0 {
		t.Errorf("pre-push off at HEAD: %+v", out)
	}
	_, a, _, ok := gated(t, r, "", PreCommit)
	if want := (model.Ladder{PreCommit: true, CommitMsg: true, TestStaged: true, SurfaceLint: true, HookFeedback: true}); !ok || a.ladder != want {
		t.Errorf("ladder at HEAD = %+v, want %+v", a.ladder, want)
	}
	r.Write(policy.FileName, "golaw: 1\nladders: {}\n")
	if _, _, _, ok := gated(t, r, "", PrePush); ok {
		t.Error("a checkout policy that does not decode leaves the ladder of the policy in force")
	}
}

func TestRefusalSpeaksThePolicyLanguageAndArmsOnlyTheRungs(t *testing.T) {
	t.Parallel()
	if got := refused("en", PrePush); got != "golaw refused the pre-push hook; the lines above name the cause and the fix." {
		t.Errorf("English refusal = %q", got)
	}
	if on(model.DefaultLadder(), "post-merge") {
		t.Error("a hook that is no rung is never armed")
	}
}

// TestGateTakesTheStagedPinOfAnUpgrade holds the commit init --update asks
// for: once the bumped golaw.yml is staged, the rungs of the binary that
// wrote it let the commit through, and before that they ask for it.
func TestGateTakesTheStagedPinOfAnUpgrade(t *testing.T) {
	t.Parallel()
	r := repository(t, pinned("0.2.0", ""))
	l := &loading{state: t.TempDir(), version: "0.3.0"}
	bumped := pinned("0.3.0", "")
	f, err := policy.Decode([]byte(bumped))
	if err != nil {
		t.Fatal(err)
	}
	record := policy.Record{Root: r.Root, PolicyDigest: policy.Digest(f), KitVersion: "0.3.0", WrittenBy: policy.WrittenByInit, WrittenAt: time.Now()}
	if err := policy.WriteRecord(l.state, record); err != nil {
		t.Fatal(err)
	}
	r.Write(policy.FileName, bumped)
	if _, out, ok := gate(t.Context(), Dependencies{Dir: r.Root, Load: l.load}, PreCommit); ok || out.Code != 2 || !strings.Contains(out.Lines[0], "commit the golaw.yml init --update wrote") {
		t.Errorf("the bump only in the checkout: %+v", out)
	}
	r.Stage(map[string]string{policy.FileName: bumped})
	if a, out, ok := gate(t.Context(), Dependencies{Dir: r.Root, Load: l.load}, PreCommit); !ok || out.Code != 0 || a.loaded.Policy.KitVersion != "0.3.0" || a.loaded.Origin != policy.OriginStaged {
		t.Errorf("the bump staged: %+v, %+v", a, out)
	}
}

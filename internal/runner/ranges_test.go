package runner_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
)

func TestRangesKeepsEveryPushedTip(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	base := repo.Commit("chore: seed", map[string]string{"base.txt": "base\n"})
	tip := repo.Branch("topic", map[string]string{"topic.txt": "topic\n"})
	defaultTip := repo.Branch("default-next", map[string]string{"default.txt": "default\n"})
	repo.Git("update-ref", "refs/remotes/origin/main", defaultTip)
	repo.Git("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	repo.Git("tag", "-a", "v1", tip, "-m", "release")
	tag := repo.Git("rev-parse", "refs/tags/v1")
	updates := []runner.RefUpdate{
		{LocalRef: "refs/heads/topic", LocalSHA: tip, RemoteRef: "refs/heads/topic", RemoteSHA: base},
		{LocalRef: "refs/heads/forced", LocalSHA: tip, RemoteRef: "refs/heads/forced", RemoteSHA: defaultTip},
		{LocalRef: "refs/heads/new", LocalSHA: tip, RemoteRef: "refs/heads/new", RemoteSHA: runner.ZeroSHA},
		{LocalRef: "(delete)", LocalSHA: runner.ZeroSHA, RemoteRef: "refs/heads/gone", RemoteSHA: defaultTip},
		{LocalRef: "refs/heads/missing", LocalSHA: tip, RemoteRef: "refs/heads/missing", RemoteSHA: strings.Repeat("f", 40)},
		{LocalRef: "refs/tags/v1", LocalSHA: tag, RemoteRef: "refs/tags/v1", RemoteSHA: runner.ZeroSHA},
	}
	want := []runner.PushRange{
		{Update: updates[0], Base: base, Tip: tip},
		{Update: updates[1], Base: defaultTip, Tip: tip},
		{Update: updates[2], Base: base, Tip: tip},
		{Update: updates[3], Tip: runner.ZeroSHA, Delete: true},
		{Update: updates[4], Tip: tip},
		{Update: updates[5], Base: base, Tip: tag},
	}
	got, err := runner.Ranges(
		t.Context(), repo.Root, "origin", updates, gittest.Isolated(),
	)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("ranges = %+v, %v", got, err)
	}
}

func TestRangesDefaultFallbacks(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		remote string
		ref    string
		base   bool
	}{
		{"near-main", "origin", "refs/remotes/origin/main", true},
		{"near-master", "origin", "refs/remotes/origin/master", true},
		{"red-no-default", "origin", "", false},
		{
			"near-url-remote", "https://example.test/repository.git",
			"refs/remotes/origin/main", false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			base := repo.Commit("chore: seed", map[string]string{"a.txt": "a\n"})
			tip := repo.Branch("topic", map[string]string{"b.txt": "b\n"})
			if tc.ref != "" {
				repo.Git("update-ref", tc.ref, base)
			}
			update := runner.RefUpdate{
				LocalRef: "HEAD", LocalSHA: tip,
				RemoteRef: "refs/heads/new", RemoteSHA: runner.ZeroSHA,
			}
			got, err := runner.Ranges(
				t.Context(), repo.Root, tc.remote,
				[]runner.RefUpdate{update}, gittest.Isolated(),
			)
			if err != nil || len(got) != 1 {
				t.Fatalf("ranges = %+v, %v", got, err)
			}
			want := ""
			if tc.base {
				want = base
			}
			if got[0].Base != want || got[0].Tip != tip {
				t.Fatalf("range = %+v", got[0])
			}
		})
	}
}

func TestRangesKeepsThePlumbingEnvironment(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	base := repo.Commit("chore: seed", map[string]string{"a.txt": "a\n"})
	tip := repo.Branch("topic", map[string]string{"b.txt": "b\n"})
	repo.Git("update-ref", "refs/remotes/origin/main", base)
	other := gittest.New(t)
	env := append(gittest.Isolated(),
		"GIT_DIR="+repo.GitDir(), "GIT_WORK_TREE="+repo.Root,
	)
	update := runner.RefUpdate{
		LocalRef: "HEAD", LocalSHA: tip,
		RemoteRef: "refs/heads/new", RemoteSHA: runner.ZeroSHA,
	}
	got, err := runner.Ranges(
		t.Context(), other.Root, "origin", []runner.RefUpdate{update}, env,
	)
	if err != nil || len(got) != 1 || got[0].Base != base {
		t.Fatalf("ranges = %+v, %v", got, err)
	}
}

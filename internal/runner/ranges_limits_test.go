package runner_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
)

func TestRangesSkipsDeletionWithoutGit(t *testing.T) {
	t.Parallel()
	update := runner.RefUpdate{
		LocalRef: "(delete)", LocalSHA: runner.ZeroSHA,
		RemoteRef: "refs/heads/gone", RemoteSHA: strings.Repeat("a", 40),
	}
	got, err := runner.Ranges(
		t.Context(), t.TempDir(), "origin", []runner.RefUpdate{update}, nil,
	)
	if err != nil || len(got) != 1 || !got[0].Delete {
		t.Fatalf("ranges = %+v, %v", got, err)
	}
}

func TestRangesDoesNotInventABaseOrTip(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	base := repo.Commit("chore: seed", map[string]string{"base.txt": "base\n"})
	repo.Git("update-ref", "refs/remotes/origin/main", base)
	blob := repo.Git("rev-parse", "HEAD:base.txt")
	repo.Git("checkout", "--orphan", "unrelated")
	tip := repo.Commit("chore: separate history", map[string]string{"other.txt": "other\n"})
	for _, tc := range []struct {
		name string
		dir  string
		tip  string
		bad  bool
	}{
		{"near-unrelated-history", repo.Root, tip, false},
		{"red-missing-local", repo.Root, strings.Repeat("f", 40), true},
		{"red-noncommit-local", repo.Root, blob, true},
		{"red-plumbing-failure", t.TempDir(), tip, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			update := runner.RefUpdate{
				LocalRef: "HEAD", LocalSHA: tc.tip,
				RemoteRef: "refs/heads/new", RemoteSHA: runner.ZeroSHA,
			}
			got, err := runner.Ranges(
				t.Context(), tc.dir, "origin",
				[]runner.RefUpdate{update}, gittest.Isolated(),
			)
			if tc.bad {
				if err == nil {
					t.Fatalf("unscannable tip accepted: %+v", got)
				}
				return
			}
			if err != nil || len(got) != 1 ||
				got[0].Base != "" || got[0].Tip != tip {
				t.Fatalf("ranges = %+v, %v", got, err)
			}
		})
	}
}

func TestRangesPrefersRemoteHead(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	base := repo.Commit("chore: seed", map[string]string{"base.txt": "base\n"})
	preferred := repo.Branch("release", map[string]string{"release.txt": "release\n"})
	repo.Git("checkout", "-q", "release")
	tip := repo.Branch("topic", map[string]string{"topic.txt": "topic\n"})
	repo.Git("update-ref", "refs/remotes/origin/main", base)
	repo.Git("update-ref", "refs/remotes/origin/release", preferred)
	repo.Git("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/release")
	update := runner.RefUpdate{
		LocalRef: "HEAD", LocalSHA: tip,
		RemoteRef: "refs/heads/topic", RemoteSHA: runner.ZeroSHA,
	}
	got, err := runner.Ranges(
		t.Context(), repo.Root, "origin",
		[]runner.RefUpdate{update}, gittest.Isolated(),
	)
	if err != nil || len(got) != 1 || got[0].Base != preferred {
		t.Fatalf("ranges = %+v, %v", got, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = runner.Ranges(
		ctx, repo.Root, "origin", []runner.RefUpdate{update}, gittest.Isolated(),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
}

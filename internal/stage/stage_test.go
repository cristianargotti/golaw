package stage_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
	"github.com/cristianargotti/golaw/internal/stage"
)

func TestScanReportsOnlyWhatTheIndexIntroduces(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		staged map[string]string
		want   []string
	}{
		"an untouched finding and a new file": {
			map[string]string{"old.md": "one " + dash + " two\nthree\n", "new.md": "a " + dash + " b\n"}, []string{"new.md"},
		},
		"a second finding in an old file": {
			map[string]string{"old.md": "one " + dash + " two\nthree " + dash + " four\n"}, []string{"old.md"},
		},
		"a clean change": {map[string]string{"clean.md": "still clean\n"}, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := gittest.New(t)
			r.Commit("docs: base", map[string]string{"old.md": "one " + dash + " two\n", "clean.md": "clean\n"})
			r.Stage(tc.staged)
			res := scan(t, r, source.IndexOptions{})
			if got := findingPaths(res.Decision); !slices.Equal(got, tc.want) {
				t.Errorf("findings at %v, want %v", got, tc.want)
			}
			if res.Input.Mode != model.ModeStaged || res.Decision.Mode != model.ModeStaged || res.Input.Snapshot != model.Snapshot(res.Index) {
				t.Errorf("input %+v, decision %+v", res.Input, res.Decision)
			}
		})
	}
}

func TestScanReadsTheIndexNeverTheWorktree(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	r.Commit("docs: base", map[string]string{"a.md": "a\n", "b.md": "b\n"})
	r.StageOnly("a.md", "clean staged\n", "dirty "+dash+" worktree\n")
	r.StageOnly("b.md", "dirty "+dash+" staged\n", "clean worktree\n")
	res := scan(t, r, source.IndexOptions{})
	if got := findingPaths(res.Decision); !slices.Equal(got, []string{"b.md"}) {
		t.Errorf("findings at %v, want b.md alone", got)
	}
	if res.Decision.Verdict != engine.VerdictFail {
		t.Errorf("verdict = %s", res.Decision.Verdict)
	}
}

func TestScanReadsAMovedFileAtItsOldPath(t *testing.T) {
	t.Parallel()
	moved := gittest.New(t)
	moved.Commit("docs: base", map[string]string{"docs/old.md": "one " + dash + " two\n", "gone.md": "x " + dash + " y\n"})
	moved.StageRename("docs/old.md", "docs/new.md")
	moved.StageDeletion("gone.md")
	if got := findingPaths(scan(t, moved, source.IndexOptions{}).Decision); len(got) != 0 {
		t.Errorf("a rename and a deletion introduce nothing: %v", got)
	}
	edited := gittest.New(t)
	edited.Commit("docs: base", map[string]string{"docs/old.md": "one " + dash + " two\nthree\nfour\nfive\n"})
	edited.StageRename("docs/old.md", "docs/new.md")
	edited.Stage(map[string]string{"docs/new.md": "one " + dash + " two\nthree\nfour\nfive " + dash + " six\n"})
	if got := findingPaths(scan(t, edited, source.IndexOptions{}).Decision); !slices.Equal(got, []string{"docs/new.md"}) {
		t.Errorf("a renamed file with one more dash = %v, want one finding at docs/new.md", got)
	}
}

func TestScanOnAnUnbornBranchJudgesEveryStagedFile(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	r.Stage(map[string]string{"a.md": "a " + dash + " b\n", "b.md": "clean\n"})
	if got := findingPaths(scan(t, r, source.IndexOptions{}).Decision); !slices.Equal(got, []string{"a.md"}) {
		t.Errorf("findings at %v, want a.md", got)
	}
}

// What a merge brought is named against HEAD, so an explicit base compares
// with that commit alone, a merge in progress included.
func TestScanComparesWithTheBaseItIsGiven(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	first := r.Commit("docs: base", map[string]string{"a.md": "clean\n"})
	r.Branch("side", map[string]string{"s.md": "s\n"})
	r.Commit("docs: a dash", map[string]string{"a.md": "a " + dash + " b\n"})
	if got := findingPaths(scan(t, r, source.IndexOptions{}).Decision); len(got) != 0 {
		t.Errorf("against HEAD nothing is staged: %v", got)
	}
	if got := findingPaths(scan(t, r, source.IndexOptions{Base: first}).Decision); !slices.Equal(got, []string{"a.md"}) {
		t.Errorf("against the first commit: %v", got)
	}
	r.MergeInProgress("side")
	if res := scan(t, r, source.IndexOptions{}); len(findingPaths(res.Decision)) != 0 || !slices.Equal(res.Brought, []string{"s.md"}) {
		t.Errorf("against HEAD during the merge: findings at %v, brought %v", findingPaths(res.Decision), res.Brought)
	}
	if res := scan(t, r, source.IndexOptions{Base: first}); !slices.Equal(findingPaths(res.Decision), []string{"a.md"}) || res.Brought != nil {
		t.Errorf("against the first commit during the merge: findings at %v, brought %v", findingPaths(res.Decision), res.Brought)
	}
}

func TestScanReadsALinkedWorktreeAndAnAlternateIndex(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	r.Commit("docs: base", map[string]string{"a.md": "a\n"})
	w := r.Worktree("feature")
	w.Stage(map[string]string{"w.md": "w " + dash + " w\n"})
	if got := findingPaths(scan(t, w, source.IndexOptions{}).Decision); !slices.Equal(got, []string{"w.md"}) {
		t.Errorf("the worktree's index: %v", got)
	}
	alternate := r.AlternateIndex()
	r.Write("alt.md", "alt "+dash+" alt\n")
	r.GitEnv([]string{"GIT_INDEX_FILE=" + alternate}, "add", "alt.md")
	if got := findingPaths(scan(t, r, source.IndexOptions{IndexFile: alternate}).Decision); !slices.Equal(got, []string{"alt.md"}) {
		t.Errorf("the alternate index: %v", got)
	}
	if got := findingPaths(scan(t, r, source.IndexOptions{}).Decision); len(got) != 0 {
		t.Errorf("the repository's own index holds nothing new: %v", got)
	}
}

// tighter is a policy in force under the core floors, so a staged file can
// weaken it and still be valid.
const tighter = "golaw: 1\nkit: {version: 0.1.0}\nchecks:\n  sizes: {file_lines: 250, function_lines: 30}\n"

func TestScanJudgesTheProposedPolicyApart(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	r.Commit("build: the policy", map[string]string{"golaw.yml": tighter})
	r.Stage(map[string]string{"golaw.yml": strings.Replace(tighter, "file_lines: 250", "file_lines: 200", 1)})
	in := input(t, r, source.IndexOptions{})
	res, err := stage.Scan(t.Context(), in)
	if err != nil || !res.Proposal.Found || res.Input.Policy != in.Loaded.Policy {
		t.Fatalf("a staged tightening = %+v, %v; the files are judged under the policy in force", res.Proposal, err)
	}
	r.Stage(map[string]string{"golaw.yml": strings.Replace(tighter, "file_lines: 250", "file_lines: 280", 1)})
	_, err = stage.Scan(t.Context(), input(t, r, source.IndexOptions{}))
	if !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), "checks.sizes.file_lines: 250 to 280") {
		t.Errorf("a staged weakening: %v", err)
	}
}

func TestScanRefusesAStagedPolicyItCannotJudge(t *testing.T) {
	t.Parallel()
	conflict := gittest.New(t)
	conflict.Commit("build: the policy", map[string]string{"golaw.yml": tighter})
	conflict.Branch("side", map[string]string{"golaw.yml": strings.Replace(tighter, "function_lines: 30", "function_lines: 20", 1)})
	conflict.Commit("build: tighten", map[string]string{"golaw.yml": strings.Replace(tighter, "file_lines: 250", "file_lines: 200", 1)})
	conflict.MergeInProgress("side")
	link := gittest.New(t)
	link.Stage(map[string]string{"target.yml": tighter})
	link.Git("update-index", "--add", "--cacheinfo", "120000,"+link.Git("rev-parse", ":target.yml")+",golaw.yml")
	for name, tc := range map[string]struct {
		r    *gittest.Repo
		says string
	}{
		"an unresolved merge": {conflict, "golaw.yml is unresolved in the index"},
		"a staged link":       {link, "read the staged golaw.yml"},
	} {
		_, err := stage.Scan(t.Context(), input(t, tc.r, source.IndexOptions{}))
		if !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), tc.says) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestScanFailsWhenGitCannotAnswer(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	r.Commit("docs: base", map[string]string{"a.md": "a\n"})
	in := input(t, r, source.IndexOptions{})
	in.Repo.Root = t.TempDir()
	if _, err := stage.Scan(t.Context(), in); !errors.Is(err, source.ErrGit) {
		t.Errorf("an index git cannot list: %v", err)
	}
	broken := input(t, r, source.IndexOptions{Base: "no-such-revision"})
	if _, err := stage.Scan(t.Context(), broken); !errors.Is(err, source.ErrGit) {
		t.Errorf("a base git cannot name: %v", err)
	}
	unregistered := input(t, r, source.IndexOptions{})
	unregistered.Registry = nil
	if _, err := stage.Scan(t.Context(), unregistered); !errors.Is(err, model.ErrUsage) {
		t.Errorf("no registry: %v", err)
	}
}

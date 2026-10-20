package stage_test

import (
	"errors"
	"slices"
	"strconv"
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

func TestScanComparesWithTheBaseItIsGiven(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	first := r.Commit("docs: base", map[string]string{"a.md": "clean\n"})
	r.Commit("docs: a dash", map[string]string{"a.md": "a " + dash + " b\n"})
	if got := findingPaths(scan(t, r, source.IndexOptions{}).Decision); len(got) != 0 {
		t.Errorf("against HEAD nothing is staged: %v", got)
	}
	if got := findingPaths(scan(t, r, source.IndexOptions{Base: first}).Decision); !slices.Equal(got, []string{"a.md"}) {
		t.Errorf("against the first commit: %v", got)
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

// goPolicy enables the go capability, so the goast checks judge the Go
// files of a case.
const goPolicy = "golaw: 1\nkit: {version: 0.1.0}\ncapabilities: [go]\n"

// goFile is a Go file of n lines: the package clause, then one declaration
// per line.
func goFile(n int) string {
	lines := make([]string, 0, n)
	lines = append(lines, "package a")
	for i := 1; i < n; i++ {
		lines = append(lines, "var v"+strconv.Itoa(i)+" = "+strconv.Itoa(i))
	}
	return strings.Join(lines, "\n") + "\n"
}

// goFunc is a Go file whose one function holds n statements, one per line.
func goFunc(n int) string {
	return "package a\n\nfunc long() {\n\tx := 0\n" + strings.Repeat("\tx++\n", n-2) + "\t_ = x\n}\n"
}

// A goast message carries a count ("file has 321 lines", "function long has
// 44 statements"), so an edit that leaves a finding the base held changes
// its message; the staged comparison reads every count as one mark, and
// any other digit, as one of a name, as itself.
func TestScanKeepsAFindingWhoseCountOnlyChanged(t *testing.T) {
	t.Parallel()
	held := "package a\n\ntype Auth1 struct{ Token string }\n\ntype Auth2 struct{ TokenID string }\n"
	moved := "package a\n\ntype Auth1 struct{ TokenID string }\n\ntype Auth2 struct{ Token string }\n"
	cases := map[string]struct {
		base, staged string
		want         []string
	}{
		"a 320-line file grown to 321":             {goFile(320), goFile(321), nil},
		"a 320-line file shortened to 310":         {goFile(320), goFile(310), nil},
		"a 45-line function shortened to 44":       {goFunc(45), goFunc(44), nil},
		"a 300-line file grown to 301":             {goFile(300), goFile(301), []string{"a.go"}},
		"a credential moved to a name a digit off": {held, moved, []string{"a.go"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := gittest.New(t)
			r.Commit("feat: base", map[string]string{"golaw.yml": goPolicy, "a.go": tc.base})
			r.Stage(map[string]string{"a.go": tc.staged})
			if got := findingPaths(scan(t, r, source.IndexOptions{}).Decision); !slices.Equal(got, tc.want) {
				t.Errorf("findings at %v, want %v", got, tc.want)
			}
		})
	}
}

// A base file over the read bound holds findings no check could read, so a
// finding at its path cannot be proven new: it is held back, never reported
// as the commit's own, and the check stays unmeasured, never clean.
func TestScanHoldsBackWhatAnUnreadBaseCannotDisprove(t *testing.T) {
	t.Parallel()
	big := "one " + dash + " two\n" + strings.Repeat("a\n", model.MaxReadBytes/2)
	short := "one " + dash + " two\n"
	cases := map[string]struct {
		base, staged map[string]string
		want         []string
		unmeasured   bool
	}{
		"an unread base shortened, its dash kept": {map[string]string{"big.md": big}, map[string]string{"big.md": short}, nil, true},
		"beside it, a new file with a dash": {
			map[string]string{"big.md": big}, map[string]string{"big.md": short, "new.md": "a " + dash + " b\n"}, []string{"new.md"}, true,
		},
		"a readable base shortened, its dash kept": {map[string]string{"big.md": short + "three\n"}, map[string]string{"big.md": short}, nil, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := gittest.New(t)
			r.Commit("docs: base", tc.base)
			r.Stage(tc.staged)
			res := scan(t, r, source.IndexOptions{})
			if got := findingPaths(res.Decision); !slices.Equal(got, tc.want) {
				t.Errorf("findings at %v, want %v", got, tc.want)
			}
			held := slices.ContainsFunc(res.Decision.Unmeasured, func(u engine.Unmeasured) bool {
				return u.Check == "text.dashes" && strings.Contains(u.Reason, "big.md")
			})
			if held != tc.unmeasured {
				t.Errorf("text.dashes held back = %v, want %v: %+v", held, tc.unmeasured, res.Decision.Unmeasured)
			}
		})
	}
}

// An exception makes a finding a note and names its record in the message
// after the engine fingerprinted it, so a kept finding keeps the
// fingerprint the engine gave it, the one golaw scan reports.
func TestScanKeepsTheFingerprintTheEngineGaveAFinding(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	excepted := "golaw: 1\nkit: {version: 0.1.0}\nexceptions: [{check: text.dashes, path: b.md, record: r.md, until: 2026-12-31}]\n"
	r.Commit("docs: base", map[string]string{"golaw.yml": excepted, "a.md": "a " + dash + " a\n"})
	r.Stage(map[string]string{"a.md": "a " + dash + " a\nmore\n", "b.md": "b " + dash + " b\n", "c.md": "c " + dash + " c\n"})
	in := input(t, r, source.IndexOptions{})
	res, err := stage.Scan(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	direct, err := engine.Dispatch(t.Context(), in.Registry, res.Input)
	if err != nil {
		t.Fatal(err)
	}
	all := engine.Decide(direct).Findings
	if len(all) != 3 || all[1].Severity != model.SeverityNote || !slices.Equal(res.Decision.Findings, all[1:]) {
		t.Errorf("findings %+v, want the engine's %+v after the one a.md carries along", res.Decision.Findings, all)
	}
}

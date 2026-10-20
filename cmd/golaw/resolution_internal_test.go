package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// resolutionCases are the commands that leave a conflict of a teammate's
// dash unmerged: the red cases, a resolution holding a dash the command
// writes without git add, a dash it writes after the separator of the
// conflict and one it writes into its own side of a conflict it wrote
// again in the diff3 style; the near misses, the
// teammate's side taken whole without git add and the conflict left as git
// wrote it, with the line ends of core.autocrlf, in the style of
// merge.conflictStyle diff3, and written again by the command in the diff3
// or the zdiff3 style or under a git -c merge.conflictStyle of its own,
// which name it in the notice, and the local side taken whole over a dash
// HEAD already held, which is unchanged; and the stated limit, a resolution
// the command writes for a conflict a rebase left, which no MERGE_HEAD
// names.
func resolutionCases(t *testing.T) []mergeCase {
	t.Helper()
	unmerged := "golaw did not measure shared.md: git left it unmerged, and golaw judges the resolution the agent writes; the pipeline judges it."
	merged := "the merge in progress brought from commits this command did not create, which ask for no fix: "
	configured := func(key, value string) func(string) {
		return func(root string) {
			gitRun(t, root, "config", key, value)
			conflicting(t, root, false)
		}
	}
	fromSide := func(root string) { conflicting(t, root, false) }
	stops := func(args ...string) func(string) { return func(root string) { gitStops(t, root, args...) } }
	then := func(first, second func(string)) func(string) { return func(root string) { first(root); second(root) } }
	resolve := func(root string) { writeFiles(t, root, map[string]string{"shared.md": "resolved \u2014 mine\n"}) }
	take := func(root string) { gitRun(t, root, "checkout", "-q", "--theirs", "shared.md") }
	keep := func(root string) { gitRun(t, root, "checkout", "-q", "--ours", "shared.md") }
	restyle := func(style string) func(string) {
		return func(root string) { gitRun(t, root, "checkout", "--conflict="+style, "--", "shared.md") }
	}
	editOwn := func(root string) {
		data, err := os.ReadFile(filepath.Join(root, "shared.md"))
		if err != nil {
			t.Fatal(err)
		}
		writeFiles(t, root, map[string]string{"shared.md": strings.Replace(string(data), "local line", "local \u2014 edited", 1)})
	}
	separator := func(root string) {
		data, err := os.ReadFile(filepath.Join(root, "shared.md"))
		if err != nil {
			t.Fatal(err)
		}
		writeFiles(t, root, map[string]string{"shared.md": strings.Replace(string(data), "=======\n", "======= \u2014 word\n", 1)})
	}
	legacy := func(root string) {
		commitFiles(t, root, map[string]string{"shared.md": "start\n"})
		sideBranch(t, root, mate(earlier), map[string]string{"shared.md": "mate line\n"})
		commitFiles(t, root, map[string]string{"shared.md": "local \u2014 legacy\n"})
	}
	return []mergeCase{
		{"a resolution the command writes without git add", fromSide, then(stops("merge", "-q", "side"), resolve), "shared.md:1", []string{merged + "theirs.md"}},
		{"the teammate's side taken whole without git add", fromSide, then(stops("merge", "-q", "side"), take), "", []string{merged + "shared.md, theirs.md"}},
		{"a conflict git wrote under core.autocrlf", configured("core.autocrlf", "true"), stops("merge", "-q", "side"), "", []string{unmerged}},
		{"a conflict git wrote in the diff3 style", configured("merge.conflictStyle", "diff3"), stops("merge", "-q", "side"), "", []string{unmerged}},
		{"a conflict the command writes again in the diff3 style", fromSide, then(stops("merge", "-q", "side"), restyle("diff3")), "", []string{unmerged}},
		{"a conflict the command writes again in the zdiff3 style", fromSide, then(stops("merge", "-q", "side"), restyle("zdiff3")), "", []string{unmerged}},
		{"a conflict of a merge under a git -c merge.conflictStyle=diff3", fromSide, stops("-c", "merge.conflictStyle=diff3", "merge", "-q", "side"), "", []string{unmerged}},
		{"a dash the command writes after the separator of a conflict", fromSide, then(stops("merge", "-q", "side"), separator), "shared.md:3", nil},
		{"a dash the command writes into its side of a diff3 conflict", fromSide, then(then(stops("merge", "-q", "side"), restyle("diff3")), editOwn), "shared.md:2", nil},
		{"the local side taken whole without git add over a dash HEAD held", legacy, then(stops("merge", "-q", "side"), keep), "", nil},
		{"a resolution of a rebase conflict, the stated limit", fromSide, then(stops("rebase", "side"), resolve), "", []string{unmerged}},
	}
}

// TestHookPostJudgesAResolutionWrittenWithoutGitAdd drives the cases of
// resolutionCases through the real binding: bytes the command wrote over a
// conflict of a merge it started are judged in the same event, while bytes
// git wrote, the conflict whatever its line ends and its style and the
// merged commit's side taken whole, are named in the notice and never fed
// back.
func TestHookPostJudgesAResolutionWrittenWithoutGitAdd(t *testing.T) {
	t.Parallel()
	for _, tc := range resolutionCases(t) {
		e := enrolledHook(t)
		tc.before(e.root)
		said := aroundCommand(t, e, tc.name, tc.during)
		judged := tc.judged != "" && fedBack(said) && strings.Contains(said, "("+tc.judged+")")
		named := true
		for _, part := range tc.said {
			named = named && strings.Contains(said, part)
		}
		if judged != (tc.judged != "") || tc.judged == "" && fedBack(said) || !named {
			t.Errorf("%s: post said %q", tc.name, said)
		}
	}
}

// TestHookAsksTheConflictOnlyForAMergeThatStops counts the git commands of
// the post event of a merge the command left stopped on a conflict: the
// bytes of the merge git makes by itself are asked once, as one git
// merge-tree, one git cat-file --batch-check that sizes the blobs and one
// git cat-file --batch that reads them, beside the merge-tree and the git
// cat-file --batch-check of the file it staged and the questions of an
// ordinary post event. The test is serial for the reason
// TestHookAsksGitEachFactOnce gives.
func TestHookAsksTheConflictOnlyForAMergeThatStops(t *testing.T) {
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	e := enrolledHook(t)
	conflicting(t, e.root, false)
	agent := agentUnderTest()
	pre := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PreToolUse", e.root, "true")), agent.Adapter, e.deps)
	gitStops(t, e.root, "merge", "-q", "side")
	if err := os.WriteFile(trace, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	post := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PostToolUse", e.root, "true")), agent.Adapter, e.deps)
	commands := gitCommandsOf(t, trace)
	count := func(name string) int {
		return len(slices.DeleteFunc(slices.Clone(commands), func(c string) bool { return !strings.Contains(c, name) }))
	}
	if pre.ExitCode != 0 || post.ExitCode != 0 || fedBack(string(post.Stdout)) || len(commands) > 10 ||
		count("merge-tree") != 2 || count("--batch-check") != 2 {
		t.Fatalf("pre %s, post %s, the post event ran %d git commands, the ceiling is 10:\n%s", pre.Stdout, post.Stdout, len(commands), strings.Join(commands, "\n"))
	}
}

// TestHookReadsNoConflictOverTheReadBound counts the git commands of the
// post event of a merge the command left stopped on the conflict of big.md,
// a file over the read bound on disk: big.md waits in the unmerged notice,
// and git is asked neither the merge nor a blob for it, no git merge-tree
// and no git cat-file --batch beside the one that reads the policy. The
// test is serial for the reason TestHookAsksGitEachFactOnce gives.
func TestHookReadsNoConflictOverTheReadBound(t *testing.T) {
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	e := enrolledHook(t)
	big := strings.Repeat("common line\n", model.MaxReadBytes/12+1)
	commitFiles(t, e.root, map[string]string{"big.md": big + "start\n"})
	sideBranch(t, e.root, mate(earlier), map[string]string{"big.md": big + "mate \u2014 line\n"})
	commitFiles(t, e.root, map[string]string{"big.md": big + "local line\n"})
	agent := agentUnderTest()
	pre := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PreToolUse", e.root, "true")), agent.Adapter, e.deps)
	gitStops(t, e.root, "merge", "-q", "side")
	if err := os.WriteFile(trace, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	post := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PostToolUse", e.root, "true")), agent.Adapter, e.deps)
	commands, said := gitCommandsOf(t, trace), string(post.Stdout)
	unmerged := "golaw did not measure big.md: git left it unmerged, and golaw judges the resolution the agent writes; the pipeline judges it."
	merged := slices.ContainsFunc(commands, func(c string) bool { return strings.Contains(c, "merge-tree") })
	reads := len(slices.DeleteFunc(slices.Clone(commands), func(c string) bool { return c != "git cat-file --batch" }))
	if pre.ExitCode != 0 || post.ExitCode != 0 || fedBack(said) || !strings.Contains(said, unmerged) || merged || reads != 1 {
		t.Fatalf("pre %s, post %s, the post event ran:\n%s", pre.Stdout, said, strings.Join(commands, "\n"))
	}
}

// TestHookAsksTheStyledConflictOnlyForBaseMarkedBytes counts the git
// commands of the post event of a merge that stops on a conflict the
// command then writes again in the diff3 style: beside the questions of a
// merge that stops, the merge git makes by itself is made once more in each
// base style, two git merge-tree, and its blobs are sized and read once,
// one more git cat-file --batch-check and one more --batch. The test is
// serial for the reason TestHookAsksGitEachFactOnce gives.
func TestHookAsksTheStyledConflictOnlyForBaseMarkedBytes(t *testing.T) {
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	e := enrolledHook(t)
	conflicting(t, e.root, false)
	agent := agentUnderTest()
	pre := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PreToolUse", e.root, "true")), agent.Adapter, e.deps)
	gitStops(t, e.root, "merge", "-q", "side")
	gitRun(t, e.root, "checkout", "--conflict=diff3", "--", "shared.md")
	if err := os.WriteFile(trace, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	post := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PostToolUse", e.root, "true")), agent.Adapter, e.deps)
	commands := gitCommandsOf(t, trace)
	count := func(name string) int {
		return len(slices.DeleteFunc(slices.Clone(commands), func(c string) bool { return !strings.Contains(c, name) }))
	}
	if pre.ExitCode != 0 || post.ExitCode != 0 || fedBack(string(post.Stdout)) || len(commands) > 14 ||
		count("merge-tree") != 4 || count("--batch-check") != 3 {
		t.Fatalf("pre %s, post %s, the post event ran %d git commands, the ceiling is 14:\n%s", pre.Stdout, post.Stdout, len(commands), strings.Join(commands, "\n"))
	}
}

// TestHookBindingReadsTheConflict reads, through the binding of the hook,
// the bytes of the merge git makes of HEAD and a teammate's branch by
// itself, the conflict of shared.md in its markers, also in the two base
// styles, and the bytes the branch holds there, and answers an error for a
// commit git does not hold and for a directory outside git.
func TestHookBindingReadsTheConflict(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	h := hookBindingFor(t, b)
	root := repository(t, true)
	conflicting(t, root, false)
	repo := hook.Repository{Root: root}
	head, side := gitOutput(t, root, "rev-parse", "HEAD"), gitOutput(t, root, "rev-parse", "side")
	conflicts, err := h.conflicts(t.Context(), repo, head, side, nil, []string{"shared.md"})
	if err != nil || !strings.HasPrefix(string(conflicts["shared.md"].Merged), "<<<<<<< ") || string(conflicts["shared.md"].Theirs) != "mate \u2014 line\n" {
		t.Fatalf("conflicts %q, %v", conflicts, err)
	}
	if _, err := h.conflicts(t.Context(), repo, head, strings.Repeat("0", 40), nil, []string{"shared.md"}); err == nil {
		t.Fatal("a commit git does not hold answered a conflict")
	}
	if _, err := h.conflicts(t.Context(), hook.Repository{Root: t.TempDir()}, head, side, nil, nil); err == nil {
		t.Fatal("a directory outside git answered a conflict")
	}
	styled, styleErr := h.restyled(t.Context(), repo, head, side, []string{"shared.md"})
	if styleErr != nil || len(styled["shared.md"]) != 2 || !strings.Contains(string(styled["shared.md"][0]), "\n||||||| ") ||
		!strings.Contains(string(styled["shared.md"][1]), "\n||||||| ") {
		t.Fatalf("styled %q, %v", styled, styleErr)
	}
	if _, err := h.restyled(t.Context(), repo, head, strings.Repeat("0", 40), []string{"shared.md"}); err == nil {
		t.Fatal("a commit git does not hold answered a styled conflict")
	}
	if _, err := h.restyled(t.Context(), hook.Repository{Root: t.TempDir()}, head, side, nil); err == nil {
		t.Fatal("a directory outside git answered a styled conflict")
	}
}

package main

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/hook"
)

// hostDiffPost is the post event of the shell tool use of shellEventAt
// whose response carries bashEditDiff as the validated host, 2.1.299,
// writes it.
func hostDiffPost(t *testing.T, cwd string, diff map[string]any) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"session_id": "session", "agent_id": "agent", "tool_use_id": "shell", "cwd": cwd,
		"hook_event_name": "PostToolUse", "tool_name": "Bash",
		"tool_input": map[string]any{"command": "true", "description": "probe"},
		"tool_response": map[string]any{
			"stdout": "", "stderr": "", "interrupted": false, "isImage": false, "noOutputExpected": false,
			"bashEditDiff": diff,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// createdDiff is the diff the host reports for a file the command created
// holding one line: the file with its hunk, and its name in changedFiles.
func createdDiff(name, line string) func(root string) map[string]any {
	return func(root string) map[string]any {
		path := filepath.Join(root, filepath.FromSlash(name))
		hunk := map[string]any{"oldStart": 0, "oldLines": 0, "newStart": 1, "newLines": 1, "lines": []string{"+" + line}}
		return map[string]any{
			"files":     []map[string]any{{"filePath": path, "hunks": []map[string]any{hunk}, "created": true}},
			"moreFiles": 0, "changedFiles": []string{path},
		}
	}
}

// modedDiff is the diff the host reports for a file whose mode alone the
// command changed: no hunk, counted in moreFiles and named in changedFiles.
func modedDiff(name string) func(root string) map[string]any {
	return func(root string) map[string]any {
		return map[string]any{"files": []map[string]any{}, "moreFiles": 1, "changedFiles": []string{filepath.Join(root, filepath.FromSlash(name))}}
	}
}

// hintCase is a command the host reports a diff for: what the repository
// holds before its pre event, what it does, the diff, the file a finding
// names when the post event feeds the dash back and what its notice says.
type hintCase struct {
	name           string
	before, during func(root string)
	diff           func(root string) map[string]any
	judged, notice string
}

// hintCases are the cases of the 2.1.299 bashEditDiff shape: a move of
// HEAD over a teammate's commit, flagged so the host diffs it, a mode change
// of a file git cannot convert, a mode change of a committed file holding a
// dash, one git ignores, and a write and a commit, the red case.
func hintCases(t *testing.T, dash string) []hintCase {
	t.Helper()
	line := strings.TrimSuffix(dash, "\n")
	theirs := func(root string) { sideBranch(t, root, mate(earlier), map[string]string{"theirs.md": dash}) }
	pushed := func(root string) { pushedByMate(t, root, map[string]string{"theirs.md": dash}) }
	git := func(args ...string) func(string) { return func(root string) { gitRun(t, root, args...) } }
	chmod := func(name string) func(string) { return func(root string) { chmodFile(t, root, name, 0o755) } }
	failing := func(root string) {
		// The committed dash file leaves the racy second too, so no git
		// status runs the failing filter on its bytes.
		stampFile(t, root, "legacy.md", time.Now().Add(-time.Hour))
		unconvertible(t, root)
	}
	committed := func(root string) { commitFiles(t, root, map[string]string{"mine.md": dash}) }
	arrived := "brought from commits this command did not create, which ask for no fix: theirs.md"
	convert := ""
	if runtime.GOOS != "windows" {
		convert = "golaw did not measure notes.md: git could not convert it to compare; the pipeline judges it."
	}
	return []hintCase{
		{"a flagged fast-forward pull of a teammate's commit", pushed, git("pull", "-q", "--ff-only", "origin", "main"), createdDiff("theirs.md", line), "", arrived},
		{"a flagged checkout of a teammate's branch", theirs, git("checkout", "-q", "side"), createdDiff("theirs.md", line), "", arrived},
		{"a reset --hard to a teammate's commit", theirs, git("reset", "-q", "--hard", "side"), createdDiff("theirs.md", line), "", arrived},
		{"a mode change of a file git cannot convert", failing, chmod("notes.md"), modedDiff("notes.md"), "", convert},
		{"a mode change of a committed file holding a dash", nil, chmod("legacy.md"), modedDiff("legacy.md"), "", ""},
		{"a mode change git ignores", git("config", "core.fileMode", "false"), chmod("legacy.md"), modedDiff("legacy.md"), "", ""},
		{"a write and a commit", nil, committed, createdDiff("mine.md", line), "mine.md", ""},
	}
}

// TestHookPostJudgesTheHostDiffAsDetectionDoes drives the cases of
// hintCases through the real binding: each file the host reports gets the
// answer change detection gives it without the report, so a file a move
// brought from a teammate's commit is named in the notice that asks for no
// fix, a file git cannot convert in the notice of its own, a mode change
// leaves a dash the tree already held alone, and only the file the command
// wrote and committed is fed back.
func TestHookPostJudgesTheHostDiffAsDetectionDoes(t *testing.T) {
	t.Parallel()
	dash := "one \u2014 two\n"
	agent := agentUnderTest()
	for _, tc := range hintCases(t, dash) {
		e := enrolledHook(t)
		commitFiles(t, e.root, map[string]string{"legacy.md": dash})
		if tc.before != nil {
			tc.before(e.root)
		}
		pre := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PreToolUse", e.root, "true")), agent.Adapter, e.deps)
		tc.during(e.root)
		post := hook.Run(t.Context(), agent.ID, strings.NewReader(hostDiffPost(t, e.root, tc.diff(e.root))), agent.Adapter, e.deps)
		said := string(post.Stdout)
		judged := tc.judged != "" && fedBack(said) && strings.Contains(said, "("+tc.judged+":1)")
		quiet := tc.judged != "" || tc.notice != "" || said == ""
		if pre.ExitCode != 0 || post.ExitCode != 0 || judged != (tc.judged != "") || tc.judged == "" && fedBack(said) ||
			!strings.Contains(said, tc.notice) || !quiet || strings.Contains(said, "legacy.md") {
			t.Errorf("%s: pre %+v, post exit %d, stdout %q, stderr %q", tc.name, pre, post.ExitCode, said, post.Stderr)
		}
	}
}

// TestHookPostJudgesTheHostDiffWithoutABaseline answers the post event of
// a command whose pre event left no baseline, so no detection classifies
// what the host reports: the file the host names is judged as it stands,
// with the notice of the missing baseline.
func TestHookPostJudgesTheHostDiffWithoutABaseline(t *testing.T) {
	t.Parallel()
	e := enrolledHook(t)
	agent := agentUnderTest()
	writeFiles(t, e.root, map[string]string{"notes.md": "one \u2014 two\n"})
	post := hook.Run(t.Context(), agent.ID, strings.NewReader(hostDiffPost(t, e.root, createdDiff("notes.md", "one \u2014 two")(e.root))), agent.Adapter, e.deps)
	said := string(post.Stdout)
	if post.ExitCode != 0 || !fedBack(said) || !strings.Contains(said, "(notes.md:1)") || !strings.Contains(said, "its pre event left no baseline") {
		t.Fatalf("a post event without a baseline: exit %d, stdout %q, stderr %q", post.ExitCode, said, post.Stderr)
	}
}

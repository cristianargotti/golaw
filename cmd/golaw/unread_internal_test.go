package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// secretLike is a variable file name the hook denies by name, assembled so
// this file names none.
func secretLike() string { return "prod" + ".tf" + "vars" }

// hintedPost is the post event of a shell command whose response names the
// files the host saw it change, as bashEditDiff carries them.
func hintedPost(t *testing.T, cwd, use string, changed []string) string {
	t.Helper()
	event := map[string]any{
		"session_id": "session", "agent_id": "agent", "tool_use_id": use, "cwd": cwd,
		"hook_event_name": "PostToolUse", "tool_name": "Bash",
		"tool_input": map[string]any{"command": "true"},
		"tool_response": map[string]any{
			"stdout": "", "stderr": "", "interrupted": false, "isImage": false,
			"bashEditDiff": map[string]any{"changedFiles": changed},
		},
	}
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestHookJudgesTheNameOfWhatItCannotRead drives post events through the
// real binding over files a command wrote past the read bound: the name
// rules still judge them, so a name the tree forbids is fed back, hinted by
// the host or found by the detection alike, with the notice of what was not
// measured, while a name the tree allows is a notice alone.
func TestHookJudgesTheNameOfWhatItCannotRead(t *testing.T) { //nolint:paralleltest // an event of 8 MiB runs alone, so its deadline measures the hook, not the tests beside it
	agent := agentUnderTest()
	for _, tc := range []struct {
		name, path, rule string
		hinted           bool
	}{
		{"a secret-like name the host reports", secretLike(), "paths.forbidden", true},
		{"a secret-like name the detection finds", secretLike(), "paths.forbidden", false},
		{"a binary document name", "docs/report.pdf", "paths.forbidden", false},
		{"a name the tree allows", "dump.sql", "", false},
	} {
		e := enrolledHook(t)
		pre := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PreToolUse", e.root, "true")), agent.Adapter, e.deps)
		writeFiles(t, e.root, map[string]string{tc.path: strings.Repeat("a", hook.MaxInputBytes+1)})
		var hints []string
		if tc.hinted {
			hints = []string{filepath.Join(e.root, filepath.FromSlash(tc.path))}
		}
		post := hook.Run(t.Context(), agent.ID, strings.NewReader(hintedPost(t, e.root, "shell", hints)), agent.Adapter, e.deps)
		said := string(post.Stdout)
		fed := strings.Contains(said, `"decision":"block"`) && tc.rule != "" && strings.Contains(said, tc.rule)
		noticed := strings.Contains(said, "golaw did not measure "+tc.path)
		if pre.ExitCode != 0 || post.ExitCode != 0 || fed != (tc.rule != "") || !noticed {
			t.Fatalf("%s: post exit %d, stdout %q, stderr %q", tc.name, post.ExitCode, said, post.Stderr)
		}
	}
}

// TestHookJudgesTheNameOfAWriteItCannotRead drives writes whose bytes the
// hook cannot hold through the real binding: an event over the input bound
// and an edit of a file over the read bound are judged by the name rules,
// so a name the tree forbids is denied, while an allowed name is allowed
// with the notice of what was not measured.
func TestHookJudgesTheNameOfAWriteItCannotRead(t *testing.T) { //nolint:paralleltest // an event of 8 MiB runs alone, so its deadline measures the hook, not the tests beside it
	e := enrolledHook(t)
	agent := agentUnderTest()
	large := strings.Repeat("a", hook.MaxInputBytes)
	for _, tc := range []struct {
		path   string
		denied bool
	}{{secretLike(), true}, {"docs/report.pdf", true}, {"data.md", false}} {
		event := hostWrite(t, e.root, filepath.Join(e.root, filepath.FromSlash(tc.path)), large, false)
		reply := hook.Run(t.Context(), agent.ID, strings.NewReader(event), agent.Adapter, e.deps)
		denied := strings.Contains(string(reply.Stdout), `"permissionDecision":"deny"`) && strings.Contains(string(reply.Stdout), "paths.forbidden")
		if reply.ExitCode != 0 || denied != tc.denied {
			t.Fatalf("a write over the input bound to %s: %+v", tc.path, reply)
		}
	}
	committed := "infra/" + secretLike()
	commitFiles(t, e.root, map[string]string{committed: strings.Repeat("a"+"\n", hook.MaxInputBytes/2+1)})
	edit := eventAt(t, "Edit", e.root, map[string]any{"file_path": filepath.Join(e.root, filepath.FromSlash(committed)), "old_string": "a", "new_string": "b", "replace_all": true})
	reply := hook.Run(t.Context(), agent.ID, strings.NewReader(edit), agent.Adapter, e.deps)
	if reply.ExitCode != 0 || !strings.Contains(string(reply.Stdout), `"permissionDecision":"deny"`) || !strings.Contains(string(reply.Stdout), "paths.forbidden") {
		t.Fatalf("an edit of a secret-like file over the read bound: %+v", reply)
	}
}

// TestHookValidatesEditsOverTheReadBound drives Edits of a committed file
// over the read bound through the real binding: an Edit without its new
// string, with an empty old string or with both schemas is cannot judge, as
// the same Edit of a small file is, while a well-formed Edit of the large
// file stays allowed with the notice naming it.
func TestHookValidatesEditsOverTheReadBound(t *testing.T) { //nolint:paralleltest // a file of 8 MiB runs alone, so its deadline measures the hook, not the tests beside it
	e := enrolledHook(t)
	agent := agentUnderTest()
	commitFiles(t, e.root, map[string]string{
		"big.sql": strings.Repeat("a", model.MaxReadBytes+1), "small.sql": "a\nend\n",
	})
	for _, tc := range []struct {
		name, reason string
		input        map[string]any
	}{
		{"missing new string", "incomplete Edit operation", map[string]any{"old_string": "a"}},
		{"empty old string", "incomplete Edit operation", map[string]any{"old_string": "", "new_string": "b"}},
		{"mixed schemas", "mixed Edit schemas", map[string]any{"old_string": "a", "new_string": "b", "edits": []map[string]any{{"old_string": "a", "new_string": "b"}}}},
		{"valid edit", "", map[string]any{"old_string": "a", "new_string": "b", "replace_all": true}},
	} {
		for _, path := range []string{"big.sql", "small.sql"} {
			tc.input["file_path"] = filepath.Join(e.root, path)
			event := eventAt(t, "Edit", e.root, tc.input)
			reply := hook.Run(t.Context(), agent.ID, strings.NewReader(event), agent.Adapter, e.deps)
			checkEditReply(t, reply, tc.name, path, tc.reason)
		}
	}
}

// checkEditReply holds the answer to an Edit of a case: exit 2 naming the
// reason when the case has one, else an allow, with the notice for the
// large file.
func checkEditReply(t *testing.T, reply hook.Reply, name, path, reason string) {
	t.Helper()
	if reason != "" {
		if reply.ExitCode != 2 || len(reply.Stdout) != 0 || !strings.Contains(reply.Stderr, "cannot judge: "+reason) {
			t.Errorf("%s of %s: exit %d, stdout %q, stderr %q", name, path, reply.ExitCode, reply.Stdout, reply.Stderr)
		}
		return
	}
	want := ""
	if path == "big.sql" {
		want = "golaw did not measure big.sql: it is over the 8 MiB the hook reads; the pipeline judges it."
	}
	if reply.ExitCode != 0 || reply.Stderr != "" || strings.Contains(string(reply.Stdout), "permissionDecision") ||
		(want == "" && len(reply.Stdout) != 0) || !strings.Contains(string(reply.Stdout), want) {
		t.Errorf("%s of %s: exit %d, stdout %q, stderr %q", name, path, reply.ExitCode, reply.Stdout, reply.Stderr)
	}
}

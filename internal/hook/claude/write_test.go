package claude_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/claude"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// writeCase is shared by the Write and Decode tests, so both entry points
// must reconstruct the same candidates.
type writeCase struct {
	name, tool, before, want string
	exists                   bool
	input                    map[string]any
	op                       model.CandidateOp
}

func writeCases() []writeCase {
	return []writeCase{
		{
			"create", "Write", "", "new", false,
			map[string]any{"content": "new"},
			model.CandidateCreate,
		},
		{
			"replace", "Write", "old", "", true,
			map[string]any{"content": ""},
			model.CandidateReplace,
		},
		{
			"edit", "Edit", "old end", "new end", true,
			map[string]any{"old_string": "old", "new_string": "new"},
			model.CandidateEdit,
		},
		{
			"all", "Edit", "old, old", "new, new", true,
			map[string]any{"old_string": "old", "new_string": "new", "replace_all": true},
			model.CandidateEdit,
		},
		{
			"sequence", "Edit", "a b", "c d", true,
			map[string]any{"edits": []map[string]any{
				{"old_string": "a", "new_string": "c"},
				{"old_string": "b", "new_string": "d"},
			}},
			model.CandidateEdit,
		},
	}
}

func TestWriteCandidates(t *testing.T) {
	t.Parallel()
	for _, tc := range writeCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			files := map[string][]byte{}
			if tc.exists {
				files["a.go"] = []byte(tc.before)
			}
			tc.input["file_path"] = filepath.Join(root, "a.go")
			got, err := claude.Write(frame(t, "PreToolUse", tc.tool, root, tc.input),
				hook.Base{Root: root}, proof.NewTree(files))
			if err != nil {
				t.Fatal(err)
			}
			checkCandidate(t, got, tc.want, tc.op, tc.before, tc.exists)
		})
	}
}

// checkCandidate also validates the event, so a decoder that builds a
// candidate the model refuses fails here and not in the engine.
func checkCandidate(t *testing.T, got hook.Request, want string, op model.CandidateOp, before string, exists bool) {
	t.Helper()
	candidate := got.Event.Candidates[0]
	if string(got.Content["a.go"]) != want || candidate.Op != op || !candidate.Reconstructed {
		t.Fatal(got)
	}
	if exists && candidate.BaseHash != hook.Hash([]byte(before)) {
		t.Fatal("base hash differs")
	}
	if err := got.Event.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestWriteCannotJudgeAndNotebook(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, tc := range []struct {
		base  string
		input map[string]any
	}{
		{"a a", map[string]any{"old_string": "missing", "new_string": "x"}},
		{"a a", map[string]any{"old_string": "a", "new_string": "x"}},
		{"a a", map[string]any{"old_string": "", "new_string": "x", "replace_all": true}},
		{"a", map[string]any{"old_string": "a"}},
		{"a a", map[string]any{"edits": []any{}}},
		{"a a", map[string]any{"content": "x"}},
		{"a", map[string]any{"old_string": "a", "new_string": "b", "edits": []map[string]any{
			{"old_string": "a", "new_string": "c"},
		}}},
	} {
		tc.input["file_path"] = filepath.Join(root, "a.go")
		if _, err := claude.Write(frame(t, "PreToolUse", "Edit", root, tc.input),
			hook.Base{Root: root}, proof.NewTree(map[string][]byte{"a.go": []byte(tc.base)})); !errors.Is(err, model.ErrCannotJudge) {
			t.Fatal(tc.input, err)
		}
	}
	for _, input := range []map[string]any{
		{"file_path": filepath.Join(root, "a.go")},
		{"file_path": filepath.Join(root, "a.go"), "content": "x", "unknown": true},
	} {
		if _, err := claude.Write(frame(t, "PreToolUse", "Write", root, input),
			hook.Base{Root: root}, proof.NewTree(nil)); !errors.Is(err, model.ErrCannotJudge) {
			t.Fatal(err)
		}
	}
	got, err := claude.Write(frame(t, "PreToolUse", "NotebookEdit", root,
		map[string]any{"notebook_path": filepath.Join(root, "a.ipynb")}),
		hook.Base{Root: root}, proof.NewTree(nil))
	if err != nil || got.Denial != "NotebookEdit is not supported by this adapter" {
		t.Fatalf("%+v, %v", got, err)
	}
}

// TestAWriteOverALargeBaseIsJudged judges a Write that replaces a file over
// the read bound on its whole content, bound to no base hash, and leaves an
// Edit of such a file, which cannot be reconstructed within the bound, to
// the pipeline with a gap naming the file.
func TestAWriteOverALargeBaseIsJudged(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	large := proof.NewTree(map[string][]byte{"big.sql": bytes.Repeat([]byte("a"), model.MaxReadBytes+1)})
	path := filepath.Join(root, "big.sql")
	got, err := claude.Write(frame(t, "PreToolUse", "Write", root, map[string]any{"file_path": path, "content": "small\n"}), hook.Base{Root: root}, large)
	if err != nil || got.Event.Candidates[0].Op != model.CandidateReplace || got.Event.Candidates[0].BaseHash != "" ||
		string(got.Content["big.sql"]) != "small\n" || len(got.Gaps) != 0 {
		t.Fatalf("a write over a large base = %+v, %v", got, err)
	}
	edit := map[string]any{"file_path": path, "old_string": "a", "new_string": "b", "replace_all": true}
	got, err = claude.Write(frame(t, "PreToolUse", "Edit", root, edit), hook.Base{Root: root}, large)
	if err != nil || len(got.Gaps) != 1 || got.Gaps[0].Path != "big.sql" || got.Event.Candidates[0].Path != "big.sql" || got.Content != nil {
		t.Fatalf("an edit of a large file = %+v, %v", got, err)
	}
}

// TestEditOverALargeBaseValidatesInput checks an Edit of a base over the
// read bound before the base is read: an incomplete Edit and one of both
// schemas are cannot judge, and a well-formed one is the gap of the file.
func TestEditOverALargeBaseValidatesInput(t *testing.T) { //nolint:paralleltest // a base of 8 MiB runs alone to bound fixture memory
	root := t.TempDir()
	large := proof.NewTree(map[string][]byte{"big.sql": bytes.Repeat([]byte("a"), model.MaxReadBytes+1)})
	for _, tc := range []struct {
		name, reason string
		input        map[string]any
	}{
		{"missing new string", "incomplete Edit operation", map[string]any{"old_string": "a"}},
		{"empty old string", "incomplete Edit operation", map[string]any{"old_string": "", "new_string": "b"}},
		{"mixed schemas", "mixed Edit schemas", map[string]any{"old_string": "a", "new_string": "b", "edits": []map[string]any{{"old_string": "a", "new_string": "b"}}}},
		{"valid edit", "", map[string]any{"old_string": "a", "new_string": "b", "replace_all": true}},
	} {
		tc.input["file_path"] = filepath.Join(root, "big.sql")
		got, err := claude.Write(frame(t, "PreToolUse", "Edit", root, tc.input), hook.Base{Root: root}, large)
		if tc.reason != "" {
			if !errors.Is(err, model.ErrCannotJudge) || !strings.Contains(err.Error(), tc.reason) {
				t.Errorf("%s: request %+v, error %v", tc.name, got, err)
			}
			continue
		}
		if err != nil || len(got.Gaps) != 1 || got.Gaps[0] != hook.OverReadBound("big.sql") ||
			len(got.Event.Candidates) != 1 || got.Event.Candidates[0].Path != "big.sql" || got.Content != nil {
			t.Errorf("%s: request %+v, error %v", tc.name, got, err)
		}
	}
}

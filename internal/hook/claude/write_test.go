package claude_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/claude"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// writeCase is one write or edit of the agent and the candidate it yields.
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
				root, proof.NewTree(files))
			if err != nil {
				t.Fatal(err)
			}
			checkCandidate(t, got, tc.want, tc.op, tc.before, tc.exists)
		})
	}
}

// checkCandidate asserts the one candidate of a write request.
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
	for _, input := range []map[string]any{
		{"old_string": "missing", "new_string": "x"},
		{"old_string": "a", "new_string": "x"},
		{"old_string": "", "new_string": "x"},
		{"old_string": "a"},
		{"edits": []any{}},
		{"content": "x"},
		{"old_string": "a", "new_string": "b", "edits": []map[string]any{
			{"old_string": "a", "new_string": "c"},
		}},
	} {
		input["file_path"] = filepath.Join(root, "a.go")
		if _, err := claude.Write(frame(t, "PreToolUse", "Edit", root, input),
			root, proof.NewTree(map[string][]byte{"a.go": []byte("a a")})); !errors.Is(err, model.ErrCannotJudge) {
			t.Fatal(err)
		}
	}
	for _, input := range []map[string]any{
		{"file_path": filepath.Join(root, "a.go")},
		{"file_path": filepath.Join(root, "a.go"), "content": "x", "unknown": true},
	} {
		if _, err := claude.Write(frame(t, "PreToolUse", "Write", root, input),
			root, proof.NewTree(nil)); !errors.Is(err, model.ErrCannotJudge) {
			t.Fatal(err)
		}
	}
	got, err := claude.Write(frame(t, "PreToolUse", "NotebookEdit", root,
		map[string]any{"notebook_path": filepath.Join(root, "a.ipynb")}),
		root, proof.NewTree(nil))
	if err != nil || got.Denial != "NotebookEdit is not supported by this adapter" {
		t.Fatalf("%+v, %v", got, err)
	}
}

package claude_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/claude"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func postFrame(t *testing.T, event, tool, root string, input, response any) []byte {
	t.Helper()
	var raw map[string]any
	if err := json.Unmarshal(frame(t, event, tool, root, input), &raw); err != nil {
		t.Fatal(err)
	}
	key := "tool_response"
	if event == "PostToolUseFailure" {
		key = "error"
	}
	raw[key] = response
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDecodeReadAndShell(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	read, err := claude.Decode(frame(t, "PreToolUse", "Read", root,
		map[string]any{"file_path": `C:\keys\private.key`}), root, nil)
	if err != nil || read.Event.Read.Path != "C:/keys/private.key" {
		t.Fatalf("%+v, %v", read, err)
	}
	for _, tool := range []string{"Bash", "PowerShell"} {
		for _, command := range []string{
			"git fetch",
			`pwsh -Command "git commit --no-verify -m x"`,
			`powershell -NoProfile -Command "git commit --no-verify -m x"`,
			`cmd /c "git commit --no-verify -m x"`,
			`sh -c 'git commit --no-verify -m x'`,
			`bash -lc 'git commit --no-verify -m x'`,
		} {
			got, err := claude.Decode(frame(t, "PreToolUse", tool, root,
				map[string]any{"command": command}), root, nil)
			if err != nil || got.Event.Shell.Command != command ||
				got.Event.Shell.Tool != tool || got.Event.Shell.Cwd != root {
				t.Fatalf("%+v, %v", got, err)
			}
		}
	}
}

func TestDecodeRoutesWritesToTheirCandidates(t *testing.T) {
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
			got, err := claude.Decode(frame(t, "PreToolUse", tc.tool, root, tc.input),
				root, proof.NewTree(files))
			if err != nil || got.Event.Shape != model.ShapeWrite || len(got.Event.Candidates) != 1 {
				t.Fatalf("%+v, %v", got, err)
			}
			checkCandidate(t, got, tc.want, tc.op, tc.before, tc.exists)
		})
	}
	root := t.TempDir()
	got, err := claude.Decode(frame(t, "PreToolUse", "NotebookEdit", root,
		map[string]any{"notebook_path": filepath.Join(root, "a.ipynb")}),
		root, proof.NewTree(nil))
	if err != nil || got.Denial != "NotebookEdit is not supported by this adapter" {
		t.Fatalf("%+v, %v", got, err)
	}
}

func TestDecodePostHintsAndFailure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, tool := range []string{"Write", "Edit", "NotebookEdit"} {
		key := "file_path"
		if tool == "NotebookEdit" {
			key = "notebook_path"
		}
		got, err := claude.Decode(postFrame(t, "PostToolUse", tool, root,
			map[string]any{key: filepath.Join(root, "a.go")},
			map[string]any{"success": true}), root, nil)
		if err != nil || !slices.Equal(got.Hints, []string{"a.go"}) {
			t.Fatalf("%+v, %v", got, err)
		}
	}
	response := map[string]any{"bashEditDiff": map[string]any{
		"changedFiles": []string{filepath.Join(root, "b.go")},
		"files":        []map[string]any{{"filePath": filepath.Join(root, "a.go")}},
	}}
	for _, tool := range []string{"Bash", "PowerShell"} {
		input := map[string]any{"command": "false"}
		got, err := claude.Decode(postFrame(t, "PostToolUse", tool, root,
			input, response), root, nil)
		if err != nil || !slices.Equal(got.Hints, []string{"a.go", "b.go"}) ||
			len(got.Event.Post.Changed) != 0 {
			t.Fatalf("%+v, %v", got, err)
		}
		got, err = claude.Decode(postFrame(t, "PostToolUseFailure", tool, root,
			input, "command failed"), root, nil)
		if err != nil || !got.Event.Post.Failed {
			t.Fatalf("%+v, %v", got, err)
		}
	}
}

func TestDecodeRejectsUnknownShapes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, data := range [][]byte{
		[]byte("{"),
		frame(t, "PreToolUse", "Read", root, map[string]any{}),
		frame(t, "PreToolUse", "Bash", root, map[string]any{}),
		frame(t, "PreToolUse", "Other", root, map[string]any{}),
		frame(t, "PostToolUse", "Bash", root, map[string]any{"command": "true"}),
		postFrame(t, "PostToolUseFailure", "Write", root,
			map[string]any{"file_path": filepath.Join(root, "a.go")}, "failed"),
		postFrame(t, "PostToolUse", "Write", root,
			map[string]any{"file_path": filepath.Join(root, "a.go")}, nil),
		postFrame(t, "PostToolUse", "Read", root,
			map[string]any{"file_path": filepath.Join(root, "a.go")}, map[string]any{"content": "x"}),
	} {
		if _, err := claude.Decode(data, root, nil); !errors.Is(err, model.ErrCannotJudge) {
			t.Fatalf("%s: %v", data, err)
		}
	}
}

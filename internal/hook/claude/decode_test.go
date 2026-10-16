package claude_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
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
		map[string]any{"file_path": `C:\keys\private.key`}), hook.Base{Root: root}, nil)
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
				map[string]any{"command": command}), hook.Base{Root: root}, nil)
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
				hook.Base{Root: root}, proof.NewTree(files))
			if err != nil || got.Event.Shape != model.ShapeWrite || len(got.Event.Candidates) != 1 {
				t.Fatalf("%+v, %v", got, err)
			}
			checkCandidate(t, got, tc.want, tc.op, tc.before, tc.exists)
		})
	}
	root := t.TempDir()
	got, err := claude.Decode(frame(t, "PreToolUse", "NotebookEdit", root,
		map[string]any{"notebook_path": filepath.Join(root, "a.ipynb")}),
		hook.Base{Root: root}, proof.NewTree(nil))
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
			map[string]any{"success": true}), hook.Base{Root: root}, nil)
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
			input, response), hook.Base{Root: root}, nil)
		if err != nil || !slices.Equal(got.Hints, []string{"a.go", "b.go"}) ||
			len(got.Event.Post.Changed) != 0 {
			t.Fatalf("%+v, %v", got, err)
		}
		got, err = claude.Decode(postFrame(t, "PostToolUseFailure", tool, root,
			input, "command failed"), hook.Base{Root: root}, nil)
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
		if _, err := claude.Decode(data, hook.Base{Root: root}, nil); !errors.Is(err, model.ErrCannotJudge) {
			t.Fatalf("%s: %v", data, err)
		}
	}
}

// realDir is a temporary directory by its real path, the form a root has.
func realDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// plant writes the files of a test and makes its symbolic links, each a
// target and a name, or skips the test where the system has none.
func plant(t *testing.T, files map[string]string, links [][2]string) {
	t.Helper()
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, pair := range links {
		if err := os.Symlink(pair[0], pair[1]); err != nil {
			t.Skipf("symbolic links are not available: %v", err)
		}
	}
}

// TestDecodeReadNamesTheEntryAndTheTarget names a read twice when a link
// stands between the name and the file: Path is the entry the name opens,
// from the root when the repository holds it, and Real the file its links
// lead to, named the same way, or by its absolute path outside.
func TestDecodeReadNamesTheEntryAndTheTarget(t *testing.T) {
	t.Parallel()
	root, outside := realDir(t), realDir(t)
	store := filepath.Join(outside, ".aws", "credentials")
	plant(t, map[string]string{
		filepath.Join(root, "payload.txt"): "plain\n", filepath.Join(root, ".env.local"): "K=v\n", store: "[default]\n",
		filepath.Join(root, "plain", "x.txt"): "plain\n",
	}, [][2]string{
		{"payload.txt", filepath.Join(root, ".env")},
		{".env.local", filepath.Join(root, "notes.txt")},
		{store, filepath.Join(root, "config.txt")},
		{filepath.Join(root, ".env.local"), filepath.Join(outside, "alias.txt")},
		{"plain", filepath.Join(root, "secrets")},
		{root, filepath.Join(outside, "proj")},
		{"..", filepath.Join(root, "plain", "secrets")},
	})
	for _, tc := range []struct{ cwd, name, path, real string }{
		{root, filepath.Join(root, ".env"), ".env", "payload.txt"},
		{root, filepath.Join(root, "secrets", "x.txt"), "secrets/x.txt", "plain/x.txt"},
		{root, filepath.Join(root, "plain", "secrets", "payload.txt"), "plain/secrets/payload.txt", "payload.txt"},
		{outside, filepath.Join(outside, "proj", "plain", "secrets", "payload.txt"), "plain/secrets/payload.txt", "payload.txt"},
		{outside, filepath.Join(outside, "proj", ".env"), ".env", "payload.txt"},
		{root, filepath.Join(root, "notes.txt"), "notes.txt", ".env.local"},
		{root, filepath.Join(root, "config.txt"), "config.txt", filepath.ToSlash(store)},
		{outside, filepath.Join(outside, "alias.txt"), filepath.ToSlash(filepath.Join(outside, "alias.txt")), ".env.local"},
		{root, filepath.Join(root, "payload.txt"), "payload.txt", ""},
	} {
		got, err := claude.Decode(frame(t, "PreToolUse", "Read", tc.cwd, map[string]any{"file_path": tc.name}), hook.Base{Root: root}, nil)
		if err != nil || got.Event.Read.Path != tc.path || got.Event.Read.Real != tc.real {
			t.Errorf("%s: %+v, %v", tc.name, got.Event.Read, err)
		}
	}
}

// TestDecodeReadIsRelativeToTheRoot names a read inside the repository by
// its path from the root, so the directories the clone sits under never
// reach the name rule, while a path outside keeps the name it was given.
func TestDecodeReadIsRelativeToTheRoot(t *testing.T) {
	t.Parallel()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "secrets", "repo")
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o700); err != nil {
		t.Fatal(err)
	}
	for target, want := range map[string]string{
		filepath.Join(root, "config", "app.yaml"): "config/app.yaml",
		filepath.Join("config", "app.yaml"):       "config/app.yaml",
		filepath.Join(base, "secrets", "db.yaml"): filepath.ToSlash(filepath.Join(base, "secrets", "db.yaml")),
	} {
		got, err := claude.Decode(frame(t, "PreToolUse", "Read", root, map[string]any{"file_path": target}), hook.Base{Root: root}, nil)
		if err != nil || got.Event.Read.Path != want {
			t.Fatalf("%s: %+v, %v", target, got.Event.Read, err)
		}
	}
}

// TestDecodeStartsAToolPathFromTheHome names the candidate of a write and
// the name of a read that start from the home the way the host opens them,
// from the root of the repository, whatever the working directory.
func TestDecodeStartsAToolPathFromTheHome(t *testing.T) {
	t.Parallel()
	base := realDir(t)
	home, root, outside := filepath.Join(base, "home"), filepath.Join(base, "repo"), filepath.Join(base, "outside")
	for _, dir := range []string{home, root, outside} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	at := hook.Base{Root: root, Home: home}
	write, err := claude.Decode(frame(t, "PreToolUse", "Write", outside,
		map[string]any{"file_path": "~/../repo/a.go", "content": "package a\n"}), at, proof.NewTree(nil))
	if err != nil || len(write.Event.Candidates) != 1 || write.Event.Candidates[0].Path != "a.go" {
		t.Fatalf("write: %+v, %v", write.Event.Candidates, err)
	}
	read, err := claude.Decode(frame(t, "PreToolUse", "Read", outside,
		map[string]any{"file_path": "~/../repo/notes.md"}), at, nil)
	if err != nil || read.Event.Read.Path != "notes.md" || read.Event.Read.Real != "" {
		t.Fatalf("read: %+v, %v", read.Event.Read, err)
	}
}

package claude_test

import (
	"bytes"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/claude"
	"github.com/cristianargotti/golaw/internal/proof"
)

// TestParseReadsTheBytesOnce decodes a write, an edit, a read, a shell
// command and a post event from one parse each: the bytes are overwritten
// once the parse returns, and its decoder still answers the request they
// held, so nothing after the parse decodes the event again.
func TestParseReadsTheBytesOnce(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "a.go")
	tree := proof.NewTree(map[string][]byte{"a.go": []byte("package a\n")})
	for _, data := range [][]byte{
		frame(t, "PreToolUse", "Write", root, map[string]any{"file_path": path, "content": "package b\n"}),
		frame(t, "PreToolUse", "Edit", root, map[string]any{"file_path": path, "old_string": "package a", "new_string": "package c"}),
		frame(t, "PreToolUse", "Read", root, map[string]any{"file_path": path}),
		frame(t, "PreToolUse", "Bash", root, map[string]any{"command": "true"}),
		postFrame(t, "PostToolUse", "Write", root, map[string]any{"file_path": path}, map[string]any{"success": true}),
	} {
		want, err := claude.Decode(bytes.Clone(data), hook.Base{Root: root}, tree)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := claude.Parse(data)
		if err != nil || parsed.Event.Target != want.Event.Target {
			t.Fatalf("%s: %+v, %v", data, parsed.Event, err)
		}
		for i := range data {
			data[i] = ' '
		}
		if got, err := parsed.Decode(hook.Base{Root: root}, tree); err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("a decode after the bytes changed: %+v, %v, want %+v", got, err, want)
		}
	}
}

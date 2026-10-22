package codex_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/codex"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

type unreadableSnapshot struct {
	model.Snapshot
}

func (unreadableSnapshot) Read(string) ([]byte, error) {
	return nil, model.ErrUnreadable
}

func TestUnreadablePatchBase(t *testing.T) {
	t.Parallel()
	data := commandPayload(t, "pre-apply-patch-add.json",
		patch("*** Update File: notes.md\n@@\n-old\n+new\n"))
	root := hooktest.Repository(t)
	parsed, err := codex.Parse(hooktest.Payload(data, root))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := unreadableSnapshot{
		Snapshot: proof.NewTree(map[string][]byte{"notes.md": []byte("old\n")}),
	}
	_, err = parsed.Decode(hook.Base{Root: root}, snapshot)
	if !errors.Is(err, model.ErrCannotJudge) {
		t.Fatalf("want cannot judge, got %v", err)
	}
}

func TestPatchSizeBoundaries(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		content string
		want    hook.Outcome
	}{
		{"file-300", "package main\n" + strings.Repeat("\n", 299), hook.Allow},
		{"file-301", "package main\n" + strings.Repeat("\n", 300), hook.Deny},
		{"function-40", "package main\nfunc work() {\n" +
			strings.Repeat("println(1)\n", 40) + "}\n", hook.Allow},
		{"function-41", "package main\nfunc work() {\n" +
			strings.Repeat("println(1)\n", 41) + "}\n", hook.Deny},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lines := strings.Split(strings.TrimSuffix(tt.content, "\n"), "\n")
			body := "*** Add File: cmd/main.go\n+" + strings.Join(lines, "\n+") + "\n"
			data := commandPayload(t, "pre-apply-patch-add.json", patch(body))
			_, result := hooktest.Evaluate(t, "codex", adapter(), data, nil)
			if result.Outcome != tt.want {
				t.Fatalf("want %s, got %+v", tt.want, result)
			}
		})
	}
}

func TestMoveProtectsBothPaths(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		from string
		to   string
		want hook.Outcome
	}{
		{"notes.md", "golaw.yml", hook.Deny},
		{"golaw.yml", "notes.md", hook.Deny},
		{"notes.md", "moved.md", hook.Allow},
	} {
		t.Run(tt.from+"-"+tt.to, func(t *testing.T) {
			t.Parallel()
			text := patch("*** Update File: " + tt.from +
				"\n*** Move to: " + tt.to + "\n@@\n-old\n+new\n")
			data := commandPayload(t, "pre-apply-patch-add.json", text)
			files := map[string][]byte{tt.from: []byte("old\n")}
			_, result := hooktest.Evaluate(t, "codex", adapter(), data, files)
			if result.Outcome != tt.want {
				t.Fatalf("want %s, got %+v", tt.want, result)
			}
		})
	}
}

func TestPatchBasePresence(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		body    string
		present bool
	}{
		{"update-absent", "*** Update File: notes.md\n@@\n-old\n+new\n", false},
		{"update-present", "*** Update File: notes.md\n@@\n-old\n+new\n", true},
		{"move-absent", "*** Update File: notes.md\n*** Move to: moved.md\n@@\n-old\n+new\n", false},
		{"move-present", "*** Update File: notes.md\n*** Move to: moved.md\n@@\n-old\n+new\n", true},
		{"delete-absent", "*** Delete File: notes.md\n", false},
		{"delete-present", "*** Delete File: notes.md\n", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			files := map[string][]byte{}
			if tt.present {
				files["notes.md"] = []byte("old\n")
			}
			data := commandPayload(t, "pre-apply-patch-add.json", patch(tt.body))
			root := hooktest.Repository(t)
			parsed, err := codex.Parse(hooktest.Payload(data, root))
			if err != nil {
				t.Fatal(err)
			}
			_, err = parsed.Decode(hook.Base{Root: root}, proof.NewTree(files))
			_, result := hooktest.Evaluate(t, "codex", adapter(), data, files)
			assertPresence(t, tt.present, err, result)
		})
	}
}

// assertPresence holds the decoder and the hook to a present base judged
// and to an absent one explained, by the decoder's error and the reason.
func assertPresence(t *testing.T, present bool, err error, result hook.Result) {
	t.Helper()
	if present {
		if err != nil || result.CannotJudge || result.Outcome != hook.Allow {
			t.Fatalf("present base: error=%v, result=%+v", err, result)
		}
		return
	}
	if !errors.Is(err, model.ErrCannotJudge) ||
		!strings.Contains(err.Error(), "notes.md") ||
		!strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("absent base error = %v", err)
	}
	if !result.CannotJudge || result.Outcome == hook.Allow ||
		!strings.Contains(result.Reason, "notes.md") ||
		!strings.Contains(result.Reason, "does not exist") {
		t.Fatalf("absent base was not explained: %+v", result)
	}
}

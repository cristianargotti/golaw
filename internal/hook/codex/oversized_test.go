package codex_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/codex"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestOversizedPatchRetainsLatePaths(t *testing.T) {
	t.Parallel()
	large := strings.Repeat("x", hook.MaxInputBytes+1)
	for _, tt := range []struct {
		name   string
		target string
		want   string
		denied bool
	}{
		{"protected", "golaw.yml", "\"permissionDecision\":\"deny\"", true},
		{"ordinary", "later.md", "additionalContext", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			text := patch("*** Add File: first.md\n+" + large +
				"\n*** Add File: " + tt.target + "\n+safe\n")
			data := commandPayload(t, "pre-apply-patch-add.json", text)
			reply := hooktest.Answer(t, "codex", adapter(), data, nil)
			if reply.ExitCode != 0 {
				t.Fatalf("exit = %d, want 0: %s", reply.ExitCode, reply.Stderr)
			}
			if !bytes.Contains(reply.Stdout, []byte(tt.want)) {
				t.Fatalf("stdout %s lacks %s", reply.Stdout, tt.want)
			}
			if tt.denied && strings.TrimSpace(reply.Stderr) == "" {
				t.Fatal("denial has no reason")
			}
		})
	}
}

func TestOversizedDecodeKeepsNamesOnly(t *testing.T) {
	t.Parallel()
	text := patch("*** Add File: notes.md\n+" +
		strings.Repeat("x", hook.MaxInputBytes+1) + "\n")
	data := commandPayload(t, "pre-apply-patch-add.json", text)
	root := hooktest.Repository(t)
	reader := iotest.OneByteReader(bytes.NewReader(hooktest.Payload(data, root)))
	parsed, err := codex.Oversized(reader)
	if err != nil {
		t.Fatal(err)
	}
	request, err := parsed.Decode(hook.Base{Root: root}, proof.NewTree(nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(request.Gaps) != 1 || request.Gaps[0].Path != "notes.md" ||
		len(request.Content) != 0 || len(request.Event.Candidates) != 1 {
		t.Fatalf("unbounded or incomplete request: %+v", request)
	}
}

func TestOversizedErrors(t *testing.T) {
	t.Parallel()
	large := strings.Repeat("x", hook.MaxInputBytes+1)
	patchData := commandPayload(t, "pre-apply-patch-add.json",
		patch("*** Add File: notes.md\n+"+large+"\n"))
	for _, tt := range []struct {
		name string
		data []byte
	}{
		{"truncated", patchData[:len(patchData)-1]},
		{"second-object", append(bytes.Clone(patchData), []byte("{}")...)},
		{"large-path", commandPayload(t, "pre-apply-patch-add.json",
			patch("*** Add File: "+large+"\n+x\n"))},
		{"broken-patch", commandPayload(t, "pre-apply-patch-add.json",
			"*** Begin Patch\n*** Add File: notes.md\n+"+large)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := codex.Oversized(bytes.NewReader(tt.data))
			if !errors.Is(err, model.ErrCannotJudge) {
				t.Fatalf("want cannot judge, got %v", err)
			}
		})
	}
}

func TestOversizedShellCannotDisappear(t *testing.T) {
	t.Parallel()
	data := commandPayload(t, "pre-bash-plain.json",
		strings.Repeat("x", hook.MaxInputBytes+1))
	reply := hooktest.Answer(t, "codex", adapter(), data, nil)
	if reply.ExitCode != 0 || !bytes.Contains(reply.Stdout, []byte("\"permissionDecision\":\"deny\"")) ||
		strings.TrimSpace(reply.Stderr) == "" {
		t.Fatalf("oversized command was not refused: %+v", reply)
	}
}

func TestLargeIrrelevantFieldDoesNotChangeCommand(t *testing.T) {
	t.Parallel()
	data := rewrite(t, corpus(t, "pre-bash-plain.json"), func(m map[string]any) {
		m["future"] = strings.Repeat("x", hook.MaxInputBytes+1)
	})
	reply := hooktest.Answer(t, "codex", adapter(), data, nil)
	if reply.ExitCode != 0 || len(reply.Stdout) != 0 || reply.Stderr != "" {
		t.Fatalf("irrelevant field changed the answer: %+v", reply)
	}
}

package codex_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/codex"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
)

// huge is a content line over the bound of the hook.
func huge(lead string) string { return lead + strings.Repeat("y", hook.MaxInputBytes) }

// overAnswer is the reply to a payload the bounded reader parses, judged
// through the core without the deadline of a whole run, so an answer over
// the bound does not depend on the load of the machine.
func overAnswer(t *testing.T, payload []byte, files map[string][]byte) hook.Reply {
	t.Helper()
	bounded := hook.Adapter{Parse: func(data []byte) (hook.Parsed, error) {
		return codex.Oversized(bytes.NewReader(data))
	}, Encode: codex.Encode}
	event, result := hooktest.Evaluate(t, "codex", bounded, payload, files)
	return codex.Encode(event, result)
}

// withinAnswer is the reply to a payload the adapter parses whole, judged
// the same way.
func withinAnswer(t *testing.T, payload []byte, files map[string][]byte) hook.Reply {
	t.Helper()
	event, result := hooktest.Evaluate(t, "codex", adapter(), payload, files)
	return codex.Encode(event, result)
}

// denied tells a reply that blocks the tool before it runs.
func denied(reply hook.Reply) bool {
	return bytes.Contains(reply.Stdout, []byte(`"permissionDecision":"deny"`))
}

// The skeleton of a patch over the bound keeps every role the grammar
// reads, so a patch the grammar accepts within the bound is named by its
// files over it, and one it refuses there is refused over it, wherever the
// long line falls.
func TestPatchSkeletonKeepsEveryRole(t *testing.T) {
	t.Parallel()
	files := map[string][]byte{"old.md": []byte("old\n")}
	for _, tt := range []struct {
		name, text string
		refused    bool
	}{
		{"add", "*** Begin Patch\n*** Add File: a.md\n+LONG\n*** End Patch\n", false},
		{"update", "*** Begin Patch\n*** Update File: old.md\n@@\n-old\n+LONG\n*** End Patch", false},
		{"move after a long line", "*** Begin Patch\n*** Add File: a.md\n+LONG\n*** Update File: old.md\n*** Move to: new.md\n@@\n-old\n+new\n*** End Patch", false},
		{"wrapped", "<<'EOF'\n*** Begin Patch\n*** Add File: a.md\n+LONG\n*** End Patch\nEOF", false},
		{"carriage returns", "*** Begin Patch\r\n*** Add File: a.md\r\n+LONG\r\n*** End Patch\r\n", false},
		{"indented header", "*** Begin Patch\n*** Add File: a.md\n+LONG\n   *** Add File: b.md\n+b\n*** End Patch", false},
		{"long context line", "*** Begin Patch\n*** Update File: old.md\n@@\n LONG\n-old\n+new\n*** End of File\n\n*** End Patch", false},
		{"short lines", "*** Begin Patch\n*** Add File: a.md\n+\n+a\n+ \n+LONG\n+b\n*** End Patch", false},
		{"long blank context", "*** Begin Patch\n*** Update File: old.md\n@@\n-old\n+new\n" + strings.Repeat(" ", 40) + "\nLONG_BLANK\n*** End Patch", false},
		{"text between hunks", "*** Begin Patch\n*** Add File: a.md\n+a\nLONG\n*** End Patch", true},
		{"tabs in a chunk", "*** Begin Patch\n*** Update File: old.md\n@@\n-old\n+new\n\t\t\tLONG_TABS\n*** End Patch", true},
		{"text after the end", "*** Begin Patch\n*** Add File: a.md\n+LONG\n*** End Patch\nLONG", true},
		{"no end marker", "*** Begin Patch\n*** Add File: a.md\n+LONG", true},
		{"short tabs in a chunk", "*** Begin Patch\n*** Update File: old.md\n@@\n-old\n+LONG\n\t\n*** End Patch", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			text := strings.NewReplacer("LONG_BLANK", strings.Repeat(" ", hook.MaxInputBytes), "LONG_TABS", strings.Repeat("\t", hook.MaxInputBytes), "LONG", huge("y")).Replace(tt.text)
			small := strings.NewReplacer("LONG_BLANK", "   ", "LONG_TABS", "\t", "LONG", "y").Replace(tt.text)
			within := withinAnswer(t, commandPayload(t, "pre-apply-patch-add.json", small), files)
			over := overAnswer(t, commandPayload(t, "pre-apply-patch-add.json", text), files)
			if denied(within) != tt.refused || denied(over) != tt.refused {
				t.Fatalf("within the bound %+v\nover it %+v", within, over)
			}
		})
	}
}

// A patch over the bound is judged by the names of its files with the
// gap of the input: golaw.yml stays protected, an absent source is named,
// a file outside the repository beside one inside it cannot be judged, a
// patch of another environment is refused, an empty one writes nothing,
// and a header longer than the hook keeps cannot be judged.
func TestPatchOverTheBoundByName(t *testing.T) {
	t.Parallel()
	files := map[string][]byte{"old.md": []byte("old\n"), "gone.md": []byte("x\n"), "kept.md": []byte("x\n")}
	for _, tt := range []struct {
		name, text, reason string
		refused            bool
	}{
		{"move to the policy", "*** Begin Patch\n*** Update File: old.md\n*** Move to: golaw.yml\n@@\n-old\n+LONG\n*** End Patch", "golaw.yml", true},
		{"absent source", "*** Begin Patch\n*** Add File: a.md\n+LONG\n*** Delete File: absent.md\n*** End Patch", "does not exist", true},
		{"outside beside inside", "*** Begin Patch\n*** Add File: a.md\n+LONG\n*** Add File: ../outside.md\n+x\n*** End Patch", "separate patch", true},
		{"environment", "*** Begin Patch\n*** Environment ID: remote\n*** Add File: a.md\n+LONG\n*** End Patch", "Environment ID", true},
		{"long header", "*** Begin Patch\n*** Add File: " + strings.Repeat("h", 70<<10) + ".md\n+LONG\n*** End Patch", "header line", true},
		{"replace delete and edit", "*** Begin Patch\n*** Add File: old.md\n+LONG\n*** Delete File: gone.md\n*** Update File: kept.md\n@@\n-x\n+y\n*** End Patch", "", false},
		{"empty patch", "*** Begin Patch\n*** End Patch\nLONG_BLANK", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			text := strings.NewReplacer("LONG_BLANK", strings.Repeat(" ", hook.MaxInputBytes), "LONG", huge("y")).Replace(tt.text)
			reply := overAnswer(t, commandPayload(t, "pre-apply-patch-add.json", text), files)
			if denied(reply) != tt.refused || !strings.Contains(reply.Stderr, tt.reason) {
				t.Fatalf("reply %+v", reply)
			}
		})
	}
}

// After a patch over the bound ran, the files it wrote are judged from the
// disk; a Bash command over the bound gets the detection of what it
// changed after it ran, and a tool the adapter does not decode cannot be
// judged.
func TestCommandOverTheBoundAfterItRan(t *testing.T) {
	t.Parallel()
	text := "*** Begin Patch\n*** Update File: main.go\n@@\n package main\n+" + huge("") + "\n*** End Patch"
	for _, tt := range []struct {
		lines int
		block bool
	}{{301, true}, {300, false}} {
		files := map[string][]byte{"main.go": []byte("package main\n" + strings.Repeat("\n", tt.lines-1))}
		reply := overAnswer(t, commandPayload(t, "post-apply-patch.json", text), files)
		if bytes.Contains(reply.Stdout, []byte(`"decision":"block"`)) != tt.block || reply.ExitCode != 0 {
			t.Fatalf("%d lines: %+v", tt.lines, reply)
		}
	}
	small := withinAnswer(t, commandPayload(t, "post-bash.json", "true"), nil)
	large := overAnswer(t, commandPayload(t, "post-bash.json", huge("true ")), nil)
	if !bytes.Equal(small.Stdout, large.Stdout) || large.ExitCode != 0 || !bytes.Contains(large.Stdout, []byte("additionalContext")) {
		t.Fatalf("post command within the bound %+v, over it %+v", small, large)
	}
	data := rewrite(t, commandPayload(t, "pre-bash-plain.json", huge("x")), func(m map[string]any) { m["tool_name"] = "spawn_agent" })
	if reply := overAnswer(t, data, nil); !denied(reply) || !strings.Contains(reply.Stderr, "unsupported Codex tool") {
		t.Fatalf("unknown tool over the bound: %+v", reply)
	}
}

// A patch over the bound whose skeleton would hold more than the bound of
// the hook, millions of short lines, cannot be judged.
func TestPatchSkeletonIsBounded(t *testing.T) {
	t.Parallel()
	text := "*** Begin Patch\n*** Add File: a.md\n" + strings.Repeat("+a\n", hook.MaxInputBytes/2) + "*** End Patch"
	_, err := codex.Oversized(bytes.NewReader(commandPayload(t, "pre-apply-patch-add.json", text)))
	if !errors.Is(err, model.ErrCannotJudge) || !strings.Contains(err.Error(), "more lines than the hook keeps") {
		t.Fatalf("want the bound named, got %v", err)
	}
}

package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
)

// credentialStruct is a Go file of package pkg with a credential in a bare
// string field, redacted by a LogValue in the same file when redacted.
func credentialStruct(redacted bool) string {
	body := "package pkg\n\nimport \"log/slog\"\n\n// Config holds the settings.\ntype Config struct {\n\tPassword string\n}\n"
	if redacted {
		return body + "\n// LogValue redacts the password.\nfunc (c Config) LogValue() slog.Value { return slog.StringValue(\"redacted\") }\n"
	}
	return body + "\nvar _ = slog.String\n"
}

// TestHookLeavesToTheNoticeOnlyWhatABoundHides drives a write of a
// credential struct beside a Go file over the read bound through the real
// binding: a struct its own file redacts is judged and allowed, one whose
// redaction could sit in the unread file is allowed with the notice naming
// that file, and the same struct with no file over the bound is denied.
func TestHookLeavesToTheNoticeOnlyWhatABoundHides(t *testing.T) { //nolint:paralleltest // an event of 8 MiB runs alone, so its deadline measures the hook, not the tests beside it
	agent := agentUnderTest()
	for _, tc := range []struct {
		name                               string
		redacted, sibling, denied, noticed bool
	}{
		{"a struct its own file redacts", true, true, false, false},
		{"a struct whose redaction the unread file may hold", false, true, false, true},
		{"the same struct with every file read", false, false, true, false},
	} {
		e := enrolledHook(t)
		if tc.sibling {
			commitFiles(t, e.root, map[string]string{"pkg/big.go": "package pkg\n\n" + strings.Repeat("// line\n", hook.MaxInputBytes/8+1)})
		}
		event := eventAt(t, "Write", e.root, map[string]any{"file_path": filepath.Join(e.root, "pkg", "conf.go"), "content": credentialStruct(tc.redacted)})
		reply := hook.Run(t.Context(), agent.ID, strings.NewReader(event), agent.Adapter, e.deps)
		said := string(reply.Stdout)
		denied := strings.Contains(said, `"permissionDecision":"deny"`) && strings.Contains(said, "goast.secret-fields")
		noticed := strings.Contains(said, "golaw did not measure pkg/big.go")
		if reply.ExitCode != 0 || denied != tc.denied || noticed != tc.noticed {
			t.Fatalf("%s: %+v", tc.name, reply)
		}
	}
}

// TestHookFindsThePathOfAWriteOverTheBoundAnywhere drives writes over the
// input bound whose path comes after their content through the real
// binding: the path is read past the content, so the write is judged by
// name and otherwise allowed with the notice naming it, as when the path
// comes first.
func TestHookFindsThePathOfAWriteOverTheBoundAnywhere(t *testing.T) { //nolint:paralleltest // an event of 8 MiB runs alone, so its deadline measures the hook, not the tests beside it
	e := enrolledHook(t)
	agent := agentUnderTest()
	large := strings.Repeat("a", hook.MaxInputBytes)
	for path, denied := range map[string]bool{"data.md": false, secretLike(): true} {
		event := hostWrite(t, e.root, filepath.Join(e.root, path), large, true)
		reply := hook.Run(t.Context(), agent.ID, strings.NewReader(event), agent.Adapter, e.deps)
		said := string(reply.Stdout)
		if reply.ExitCode != 0 || strings.Contains(said, `"permissionDecision":"deny"`) != denied || !denied && !strings.Contains(said, "golaw did not measure "+path) {
			t.Fatalf("%s after its content: %+v", path, reply)
		}
	}
}

// TestHookDetectsTheChangesOfAPostEventOverTheBound drives a shell post
// event whose response is over the input bound through the real binding:
// its detection needs no byte of the event, so a dash the command wrote is
// fed back as for any post event.
func TestHookDetectsTheChangesOfAPostEventOverTheBound(t *testing.T) { //nolint:paralleltest // an event of 8 MiB runs alone, so its deadline measures the hook, not the tests beside it
	e := enrolledHook(t)
	agent := agentUnderTest()
	if pre := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PreToolUse", e.root, "true")), agent.Adapter, e.deps); pre.ExitCode != 0 {
		t.Fatalf("pre: %+v", pre)
	}
	writeFiles(t, e.root, map[string]string{"notes.md": "one \u2014 two\n"})
	post := fmt.Sprintf(`{"session_id":"session","agent_id":"agent","cwd":%q,"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"true"},"tool_response":{"stdout":%q,"stderr":"","interrupted":false,"isImage":false},"tool_use_id":"shell"}`,
		e.root, strings.Repeat("a", hook.MaxInputBytes))
	reply := hook.Run(t.Context(), agent.ID, strings.NewReader(post), agent.Adapter, e.deps)
	if said := string(reply.Stdout); reply.ExitCode != 0 || !strings.Contains(said, `"decision":"block"`) || !strings.Contains(said, "text.dashes") {
		t.Fatalf("a post event over the bound: %+v", reply)
	}
}

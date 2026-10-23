package main

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/install"
)

// m4WriteCapture records the keys of the undocumented Write payload for the
// repository the payload names, as the hook process does before its allow.
func m4WriteCapture(t *testing.T, s *suite, c command) {
	t.Helper()
	var payload struct {
		Cwd string `json:"cwd"`
	}
	if err := json.Unmarshal([]byte(c.input), &payload); err != nil {
		t.Fatal(err)
	}
	_, identity, err := install.ProjectIdentity(filepath.FromSlash(payload.Cwd))
	if err != nil {
		t.Fatal(err)
	}
	record, err := json.Marshal(doctor.CaptureRecord{Identity: identity, Captures: []hook.Capture{
		{Event: cursorPreTool, Tool: "Write", Keys: []string{m4UnknownKey}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	checkedWrite(t, agents.CapturePath(filepath.Join(s.home, "state", "golaw"), agentCursor, identity), string(record))
}

// m4FakeCursor answers as golaw's Cursor adapter answers the corpus and the
// sample fixtures: the read of .env refused without an agent message, a
// payload naming RED refused before the tool or reported after it, every
// other one allowed, the undocumented Write captured first when capture
// holds.
func m4FakeCursor(t *testing.T, s *suite, capture bool) func(command) result {
	t.Helper()
	return func(c command) result {
		post := strings.Contains(c.input, `"postToolUse`)
		red := strings.Contains(c.input, "RED")
		switch {
		case strings.Contains(c.input, `"beforeReadFile"`):
			return result{stdout: `{"permission":"deny","user_message":"` + sampleHuman + `"}` + "\n"}
		case red && post:
			return result{stdout: `{"additional_context":"` + sampleReason + `"}` + "\n"}
		case red:
			return result{stdout: `{"permission":"deny","user_message":"` + sampleHuman + `","agent_message":"` + sampleReason + `"}` + "\n"}
		case post:
			return result{stdout: "{}\n"}
		}
		if capture && strings.Contains(c.input, m4UnknownKey) {
			m4WriteCapture(t, s, c)
		}
		return result{stdout: `{"permission":"allow"}` + "\n"}
	}
}

// m4CursorSample is a Linux replay of the real Cursor corpus and two sample
// routes.
func m4CursorSample(t *testing.T, capture bool) (*suite, *[]command) {
	t.Helper()
	s := m4ReplaySuite(t)
	s.root = m4SampleRoot(t, m4CursorCorpus, m4CursorManifest,
		`{"conversation_id":"conversation","generation_id":"generation","hook_event_name":"beforeShellExecution","workspace_roots":["/project"],"command":"echo MARK","cwd":"/project"}`,
		`{"conversation_id":"conversation","generation_id":"generation","hook_event_name":"postToolUse","workspace_roots":["/project"],"tool_name":"Shell","tool_input":{"command":"echo MARK"},"tool_output":"","tool_use_id":"tool","cwd":"/project"}`)
	seen := &[]command{}
	answer := m4FakeCursor(t, s, capture)
	s.execute = m4Script(s, func(c command) result {
		*seen = append(*seen, c)
		return answer(c)
	})
	return s, seen
}

func TestM4CursorRowsReplayTheCorpusAndTheRoutes(t *testing.T) {
	t.Parallel()
	s, seen := m4CursorSample(t, true)
	want := []string{
		"before-read-file PASS", "before-shell-execution PASS", "post-tool-use-failure-shell PASS", "post-tool-use-shell PASS",
		"pre-tool-use-shell PASS", "pre-tool-use-write-unknown PASS", "shell-red PASS", "shell-near PASS", "post-red PASS", "post-near PASS",
	}
	if got := m4RowNames(s.m4CursorRows(t.Context(), false)); !slices.Equal(got, want) {
		t.Fatalf("rows = %q", got)
	}
	cursor := filepath.Join(s.home, ".cursor")
	for _, c := range *seen {
		if c.dir != cursor {
			t.Fatalf("a Cursor hook ran in %s, not in %s", c.dir, cursor)
		}
	}
}

func TestM4CursorRowsRequireTheCaptureBeforeTheAllow(t *testing.T) {
	t.Parallel()
	s, _ := m4CursorSample(t, false)
	rows := s.m4CursorRows(t.Context(), false)
	index := slices.IndexFunc(rows, func(item row) bool { return item.name == "pre-tool-use-write-unknown" })
	if index < 0 || rows[index].status != statusFail || !strings.Contains(rows[index].detail, "read acceptance file") {
		t.Fatalf("a write allowed without its capture = %+v", rows)
	}
	repo := t.TempDir()
	checkedWrite(t, filepath.Join(s.home, "state", "golaw", agentCursor, "captures", "x.json"), "{")
	if err := s.m4Captured(repo); err == nil {
		t.Fatal("a repository without a capture record passed")
	}
	m4WriteCapture(t, s, command{input: `{"cwd":"` + filepath.ToSlash(repo) + `"}`})
	if err := s.m4Captured(repo); err != nil {
		t.Fatalf("a recorded capture = %v", err)
	}
}

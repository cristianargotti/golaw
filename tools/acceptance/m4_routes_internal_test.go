package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// m4Script answers the shells of a replay through answer and every other
// command, git and golaw init among them, through the simulated tools.
func m4Script(s *suite, answer func(c command) result) executor {
	base := s.execute
	return func(ctx context.Context, c command) result {
		if c.program == s.git || c.program == s.binary || c.program == s.goTool {
			return base(ctx, c)
		}
		return answer(c)
	}
}

// m4ReplaySuite is a prepared suite replaying for Linux with golaw's Codex
// and Cursor hooks installed under its home.
func m4ReplaySuite(t *testing.T) *suite {
	t.Helper()
	s := testSuite(t)
	s.m2OS = "linux"
	m4Installed(t, s.home, s.binary, s.m2OS)
	return s
}

// m4WriteFixture writes the files of a fixture directory.
func m4WriteFixture(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		checkedWrite(t, filepath.Join(dir, filepath.FromSlash(rel)), body)
	}
}

func TestM4ReadManifestRefusesAnIncompleteRoute(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, tc := range []struct{ name, body, want string }{
		{"near two routes", "checks:\n  - id: codex.bash\n    code: bash\n    red: testdata/bash/red\n    near: testdata/bash/near\n", ""},
		{"red an unknown key", "checks:\n  - id: codex.bash\n    code: bash\n    red: r\n    near: n\n    extra: x\n", "decode"},
		{"red no route", "checks: []\n", "lists no route"},
		{"red a route without a code", "checks:\n  - id: codex.bash\n    red: r\n    near: n\n", "incomplete route"},
	} {
		path := filepath.Join(dir, "manifest.yml")
		checkedWrite(t, path, tc.body)
		routes, err := m4ReadManifest(path)
		if tc.want == "" && (err != nil || len(routes) != 1 || routes[0].Near != "testdata/bash/near") ||
			tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
			t.Errorf("%s: %+v, %v", tc.name, routes, err)
		}
	}
	if _, err := m4ReadManifest(filepath.Join(dir, "absent.yml")); err == nil {
		t.Fatal("an absent manifest passed")
	}
}

func TestM4LoadFixtureSplitsTheProofConvention(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m4WriteFixture(t, dir, map[string]string{
		"payload.json": `{"cwd":"/project"}`, "pre.json": `{"pre":true}`,
		"before/notes.md": "safe\n", "notes.md.esc": "one\\u2014two\n", "sub/kept.txt": "kept\n",
	})
	fixture, err := m4LoadFixture(dir)
	if err != nil {
		t.Fatal(err)
	}
	dash := "one" + string(rune(0x2014)) + "two\n"
	if string(fixture.payload) != `{"cwd":"/project"}` || string(fixture.pre) != `{"pre":true}` ||
		string(fixture.before["notes.md"]) != "safe\n" || string(fixture.after["notes.md"]) != dash ||
		string(fixture.after["sub/kept.txt"]) != "kept\n" || len(fixture.after) != 2 {
		t.Fatalf("fixture = %+v", fixture)
	}
	empty := t.TempDir()
	checkedWrite(t, filepath.Join(empty, "notes.md"), "x")
	if _, err := m4LoadFixture(empty); err == nil || !strings.Contains(err.Error(), "holds no payload.json") {
		t.Fatalf("a fixture without a payload = %v", err)
	}
	if _, err := m4LoadFixture(filepath.Join(dir, "absent")); err == nil {
		t.Fatal("an absent fixture passed")
	}
}

func TestM4UnescapeDecodesTheSixCharacters(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]string{
		"one\\u2014two":    "one" + string(rune(0x2014)) + "two",
		"keep \\\\u2014":   "keep \\u2014",
		"short \\u20":      "short \\u20",
		"other \\x41 \\n":  "other \\x41 \\n",
		"upper \\u00E1 ok": "upper " + string(rune(0xe1)) + " ok",
	} {
		if got := string(m4Unescape([]byte(text))); got != want {
			t.Errorf("%q = %q, want %q", text, got, want)
		}
	}
}

func TestM4RouteOfReachesGolawForEveryPayload(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ agent, payload, event, tool string }{
		{agentCodex, `{"hook_event_name":"PostToolUse","tool_name":"apply_patch"}`, eventPost, "apply_patch"},
		{agentCodex, `{"hook_event_name":"beforeShellExecution","command":"git status"}`, eventPre, toolBash},
		{agentCodex, "not-json", eventPre, toolBash},
		{agentCursor, `{"hook_event_name":"beforeReadFile","file_path":"/project/.env"}`, "beforeReadFile", ""},
		{agentCursor, `{"hook_event_name":"postToolUseFailure","tool_name":"Shell"}`, "postToolUseFailure", "Shell"},
		{agentCursor, `{"hook_event_name":"PreToolUse","tool_name":"Write"}`, "beforeShellExecution", ""},
	} {
		if event, tool := m4RouteOf(tc.agent, []byte(tc.payload)); event != tc.event || tool != tc.tool {
			t.Errorf("%s %s = %s %q", tc.agent, tc.payload, event, tool)
		}
	}
	if got := string(m4Payload([]byte(`{"cwd":"/project","file_path":"/project/.env"}`), filepath.Join("C:", "repo"))); strings.Contains(got, m4ProjectRoot) {
		t.Fatalf("payload = %s", got)
	}
}

func TestM4ReplaySendsThePreEventFirst(t *testing.T) {
	t.Parallel()
	s := m4ReplaySuite(t)
	var seen []string
	s.execute = m4Script(s, func(c command) result {
		notes, err := os.ReadFile(filepath.Join(c.dir, "notes.md"))
		_, goneErr := os.Stat(filepath.Join(c.dir, "gone.txt"))
		seen = append(seen, c.input+"|"+string(notes)+"|"+boolText(err == nil && goneErr == nil))
		return result{}
	})
	fixture := m4Fixture{
		payload: []byte(`{"hook_event_name":"PostToolUse","tool_name":"Bash","cwd":"/project"}`),
		pre:     []byte(`{"hook_event_name":"PreToolUse","tool_name":"Bash","cwd":"/project"}`),
		before:  map[string][]byte{"notes.md": []byte("safe\n"), "gone.txt": []byte("x")},
		after:   map[string][]byte{"notes.md": []byte("changed\n")},
	}
	event, replies, err := s.m4Replay(t.Context(), agentCodex, "m4-replay", fixture)
	if err != nil || event != eventPost || len(replies) != 1 || len(seen) != 2 {
		t.Fatalf("event %s, replies %+v, seen %q: %v", event, replies, seen, err)
	}
	repo := filepath.ToSlash(filepath.Join(s.base, dirRepos, "m4-replay"))
	if !strings.Contains(seen[0], `"PreToolUse"`) || !strings.Contains(seen[0], repo) || !strings.HasSuffix(seen[0], "|safe\n|true") ||
		!strings.Contains(seen[1], `"PostToolUse"`) || !strings.HasSuffix(seen[1], "|changed\n|false") {
		t.Fatalf("seen = %q", seen)
	}
	s.execute = m4Script(s, func(command) result { return codexDeny() })
	if _, _, err := s.m4Replay(t.Context(), agentCodex, "m4-replay-denied", fixture); err == nil || !strings.Contains(err.Error(), "the pre event of m4-replay-denied") {
		t.Fatalf("a denied pre event = %v", err)
	}
}

// TestM4JudgedReadsEveryShellAsItIs holds a Codex denial through every
// shell to the bytes of the denial, exit 0 with the deny JSON under
// PowerShell too, and refuses another exit through any shell (the stated
// limit the review of the plan had added went with the owner's answer of
// 2026-10-21 to decision 2 of the plan; not replayed).
func TestM4JudgedReadsEveryShellAsItIs(t *testing.T) {
	t.Parallel()
	denied := codexDeny()
	failed, other := denied, denied
	failed.code, other.code = 1, 2
	for _, row := range []struct {
		name    string
		replies []m4Reply
		ok      bool
	}{
		{"near the denial through both shells", []m4Reply{{shell: "cmd.exe", got: denied}, {shell: "powershell.exe", got: denied}}, true},
		{"red exit 1 through PowerShell, a failed hook", []m4Reply{{shell: "cmd.exe", got: denied}, {shell: "powershell.exe", got: failed}}, false},
		{"red exit 2 through cmd", []m4Reply{{shell: "cmd.exe", got: other}}, false},
	} {
		if err := m4Judged(agentCodex, eventPre, m4Deny, row.replies); (err == nil) != row.ok {
			t.Errorf("%s: %v", row.name, err)
		}
	}
}

// boolText spells a truth value for a recorded line.
func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

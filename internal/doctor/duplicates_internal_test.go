package doctor

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRunsGolawHookRecognizesTheBinaryAndItsSubcommand(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		command string
		args    []string
		want    bool
	}{
		{"golaw", []string{"hook", "claude"}, true},
		{"golaw hook claude", nil, true},
		{"'/opt/bin/golaw' hook cursor", nil, true},
		{`C:\Tools\golaw.exe hook claude`, nil, true},
		{"/usr/bin/env golaw hook claude", nil, true},
		{"golaw", []string{"version"}, false},
		{"golaw-hook claude", nil, false},
		{"/opt/notgolaw hook claude", nil, false},
		{"echo golaw hook claude", nil, false},
		{"", nil, false},
	} {
		if got := runsGolawHook(commandWords(tc.command, tc.args)); got != tc.want {
			t.Errorf("%q %q = %v", tc.command, tc.args, got)
		}
	}
}

// homeAndRepository is a temporary home and repository a report reads.
func homeAndRepository(t *testing.T, agent, scope string) Input {
	t.Helper()
	home, root := t.TempDir(), t.TempDir()
	return Input{Agent: agent, Scope: scope, Home: home, Root: root, ConfigDir: filepath.Join(home, "."+agent)}
}

func TestCollectDuplicatesFindsTheLegacySkillBesideTheShared(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"user", scopeProject} {
		input := homeAndRepository(t, agentCodex, scope)
		legacy := filepath.Join(input.ConfigDir, "skills", "golaw", skillName)
		writeFile(t, legacy, "skill\n")
		if got := collectDuplicates(input, ""); got != nil {
			t.Fatalf("%s: a legacy skill alone = %+v", scope, got)
		}
		writeFile(t, skillPath(input), "skill\n")
		want := []duplicateFact{{Kind: duplicateSkill, Path: legacy, Beside: skillPath(input)}}
		if got := collectDuplicates(input, ""); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: duplicates = %+v", scope, got)
		}
	}
	if !strings.HasPrefix(skillPath(Input{Scope: scopeProject, Root: "repo", Home: "home"}), "repo") {
		t.Fatal("a project skill outside the repository")
	}
}

func TestCollectDuplicatesFindsAClaudeHookCursorImports(t *testing.T) {
	t.Parallel()
	input := homeAndRepository(t, agentCursor, "user")
	golaw := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"golaw","args":["hook","claude"]}]}]}}`
	other := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"lint-staged"}]}]}}`
	writeFile(t, filepath.Join(input.Home, ".claude", "settings.json"), other)
	writeFile(t, filepath.Join(input.Root, ".claude", "settings.local.json"), golaw)
	writeFile(t, filepath.Join(input.Root, ".claude", "settings.json"), "{")
	if got := collectDuplicates(input, ""); got != nil {
		t.Fatalf("Cursor not installed = %+v", got)
	}
	want := []duplicateFact{{Kind: duplicateHook, Path: filepath.Join(input.Root, ".claude", "settings.local.json"), Beside: "hooks.json"}}
	if got := collectDuplicates(input, "hooks.json"); !reflect.DeepEqual(got, want) {
		t.Fatalf("duplicates = %+v", got)
	}
	input.Agent = agentCodex
	if got := collectDuplicates(input, "hooks.json"); got != nil {
		t.Fatalf("Codex imports no Claude settings: %+v", got)
	}
	input.Agent = agentClaude
	if got := collectDuplicates(input, "hooks.json"); got != nil {
		t.Fatalf("the Claude plugin = %+v", got)
	}
}

func TestDuplicateLinesNameEachCopyOrNone(t *testing.T) {
	t.Parallel()
	facts := installedFacts(agentCursor, "user")
	if lines := duplicateLines(facts); len(lines) != 1 || lines[0].Code != codeNoDuplicate || lines[0].State != stateOK {
		t.Fatalf("none = %+v", lines)
	}
	facts.Duplicates = []duplicateFact{
		{Kind: duplicateHook, Path: "settings.json", Beside: "hooks.json"},
		{Kind: duplicateSkill, Path: "legacy/SKILL.md", Beside: "shared/SKILL.md"},
	}
	lines := duplicateLines(facts)
	if len(lines) != 2 || lines[0].State != stateRed || lines[0].Code != codeDuplicateHook || !strings.Contains(lines[0].Detail, "runs the hook twice") ||
		lines[1].State != stateUnknown || lines[1].Code != codeSkillDuplicate || !strings.Contains(lines[1].Detail, "remove the legacy copy") {
		t.Fatalf("lines = %+v", lines)
	}
	if duplicateLines(installedFacts(agentClaude, "user")) != nil {
		t.Fatal("the Claude report carries a duplicate line")
	}
}

func TestCollectAgentFindsDuplicatesOfAnInstalledCursor(t *testing.T) {
	t.Parallel()
	in := installCursor(t)
	in.input.Home, in.input.Root, in.input.ConfigDir = in.dir, t.TempDir(), filepath.Join(in.dir, ".cursor")
	writeFile(t, filepath.Join(in.dir, ".claude", "settings.json"), `{"hooks":{"PreToolUse":[{"hooks":[{"command":"golaw hook claude"}]}]}}`)
	facts := collectAgent(in.input)
	if len(facts.Duplicates) != 1 || facts.Duplicates[0].Beside != in.hooks {
		t.Fatalf("duplicates = %+v", facts.Duplicates)
	}
}

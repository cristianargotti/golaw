package cicheck_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/agents"
)

// m7InstructionPaths are the skill, its installed copy, the instruction
// block and the editor rule, named through the registered agent ids the
// way the M4 test names them.
func m7InstructionPaths(t *testing.T) ([]string, []string) {
	t.Helper()
	ids := agents.IDs()
	if len(ids) != 3 {
		t.Fatal("expected three registered agents")
	}
	return []string{"agents/SKILL.md", "agents/" + ids[0] + "/skills/golaw/SKILL.md"},
		[]string{"agents/" + ids[1] + "/AGENTS.md", "agents/" + ids[2] + "/golaw.mdc"}
}

// m7SkillRule and m7SkillPointer are the rule of M7.9.5 and its pointer
// sentence, the first two paragraphs of the skill.
const (
	m7SkillRule    = "Run `golaw map` before you explore. Create a port, adapter, use case or handler with `golaw new`; never edit a generated file, edit its contract and the hook regenerates it. Fill the `golaw:hole` markers and their red tests; `golaw gen --check` must pass before you finish."
	m7SkillPointer = "Queries go through the pinned sqlc and HTTP contracts through the pinned oapi-codegen, both run by `golaw gen`; write the migration and the query before `golaw new adapter sql`."
	m7BlockLine    = "Run golaw map before you explore an enrolled repository; create kinds with golaw new, fill their golaw:hole markers, and pass golaw gen --check before you finish."
)

func TestTheSkillOpensWithTheGeneratorRule(t *testing.T) {
	t.Parallel()
	skills, pointers := m7InstructionPaths(t)
	for _, path := range skills {
		data := kitBytes(t, path)
		_, body, found := strings.Cut(string(data), "# Go engineering with golaw\n\n")
		if !found || !strings.HasPrefix(body, m7SkillRule+"\n\n"+m7SkillPointer+"\n") {
			t.Errorf("%s does not open with the generator rule and its pointer", path)
		}
		if strings.Contains(string(data), "run go generate ./...") {
			t.Errorf("%s still carries the first wave rule", path)
		}
		if reason := inspectSkill(path, data); reason != "" {
			t.Errorf("%s: %s", path, reason)
		}
	}
	for _, path := range pointers {
		requireText(t, path, m7BlockLine)
		if reason := inspectProseBudget(path, kitBytes(t, path)); reason != "" {
			t.Errorf("%s: %s", path, reason)
		}
	}
}

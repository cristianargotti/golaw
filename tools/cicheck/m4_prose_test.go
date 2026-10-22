package cicheck_test

import (
	"bytes"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/agents"
)

func TestM4InstructionAssets(t *testing.T) {
	t.Parallel()
	ids := agents.IDs()
	if len(ids) != 3 {
		t.Fatal("expected three registered agents")
	}
	const sourcePath = "agents/SKILL.md"
	source := kitBytes(t, sourcePath)
	copyPath := "agents/" + ids[0] + "/skills/golaw/SKILL.md"
	if !bytes.Equal(source, kitBytes(t, copyPath)) {
		t.Fatal("skill copy differs from its source")
	}
	requireText(t, sourcePath,
		"Installed guards deny recognized violations only on enforced routes.",
		"Capture, feedback and installation alone do not prove enforcement.",
		"Use golaw doctor to distinguish installed, trusted and active hooks.",
		"Write and Delete payloads are captured by key name and allowed.",
		"Run golaw scan --staged before finishing staged work.",
		"Ask before pushing or opening a pull request.",
	)
	blockPath := "agents/" + ids[1] + "/AGENTS.md"
	rulePath := "agents/" + ids[2] + "/golaw.mdc"
	for _, path := range []string{sourcePath, copyPath, blockPath, rulePath} {
		if reason := inspectProseBudget(path, kitBytes(t, path)); reason != "" {
			t.Errorf("%s: %s", path, reason)
		}
	}
	requireText(t, blockPath,
		"<!-- golaw:begin -->",
		"~/.agents/skills/golaw/SKILL.md",
		".agents/skills/golaw/SKILL.md",
		"installed, trusted and active hooks",
		"local checks remain optional",
		"<!-- golaw:end -->",
	)
	requireText(t, rulePath,
		"alwaysApply: true",
		".agents/skills/golaw/SKILL.md",
		"compatibility preview",
		"Write and Delete capture is not enforcement",
		"post feedback is advisory",
		"local checks remain optional",
	)
}

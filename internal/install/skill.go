package install

import "path/filepath"

const (
	scopeUser    = "user"
	scopeProject = "project"
	skillSource  = "agents/SKILL.md"
	receiptName  = "receipt.json"
)

// SkillPath is where the one shared skill of the hooks file agents lives:
// the directory both hosts document, under the home of the person by
// default and under the repository with --scope project, so every agent
// installed at one scope reads the same file and each receipt can name it.
func SkillPath(options Options) string {
	base := options.Home
	if options.Scope == scopeProject {
		base = options.Root
	}
	return filepath.Join(base, ".agents", "skills", kitName, "SKILL.md")
}

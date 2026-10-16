package cicheck_test

import (
	"bytes"
	"strings"

	"gopkg.in/yaml.v3"
)

func lineCount(data []byte) int {
	lines := bytes.Count(data, []byte("\n"))
	if len(data) != 0 && data[len(data)-1] != '\n' {
		lines++
	}
	return lines
}

func inspectSkill(path string, data []byte) string {
	if lineCount(data) > 100 || len(data) > 10000 {
		return "Skill exceeds its size budget."
	}
	if reason := inspectPublicText(path, data); reason != "" {
		return reason
	}
	text := string(data)
	if !strings.HasPrefix(text, "---\n") {
		return "Skill frontmatter is missing."
	}
	front, _, found := strings.Cut(text[4:], "\n---\n")
	if !found {
		return "Skill frontmatter is incomplete."
	}
	var header struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(front), &header); err != nil {
		return "Skill frontmatter is invalid."
	}
	if header.Name != "golaw" ||
		!strings.Contains(header.Description, "Go") ||
		!strings.Contains(header.Description, "golaw") ||
		!strings.Contains(header.Description, "repository") {
		return "Skill identity or trigger is invalid."
	}
	return ""
}

func skillRule() auditRule {
	return auditRule{
		id: "docs.skill", rule: "rules/4.5@1",
		match: func(path string) bool {
			return path == "agents/SKILL.md" ||
				strings.HasSuffix(path, "/skills/golaw/SKILL.md")
		},
		inspect: inspectSkill,
	}
}

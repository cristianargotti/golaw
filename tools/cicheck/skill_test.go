package cicheck_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalSkill(t *testing.T) {
	t.Parallel()
	source := kitBytes(t, "agents/SKILL.md")
	if reason := inspectSkill("agents/SKILL.md", source); reason != "" {
		t.Fatal(reason)
	}
	requireText(t, "agents/SKILL.md",
		"existing platform capability",
		"failures.Prefix",
		"errors.Is",
		"one use case per file",
		"400 changed lines",
		"Ask before pushing or opening a pull request.",
		"Run golaw scan . before finishing.",
		"When golaw scan --staged exists in your version, run it before finishing staged work.",
		"The rules page carries the prose mapping; read docs/checklists.md of the kit at the same tag when it exists.",
	)
	copies, err := filepath.Glob("../../agents/*/skills/golaw/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(copies) == 0 {
		t.Fatal("installed skill copy is absent")
	}
	for _, path := range copies {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(source, data) {
			t.Errorf("skill copy differs: %s", path)
		}
	}
	requireText(t, "docs/rules.md", "Prose mapping", "errors.Is", "20 statement minimum",
		"goast.function-length", "goast.sonar-identical-functions")
}

func TestSkillBudgetBoundaries(t *testing.T) {
	t.Parallel()
	header := "---\nname: golaw\ndescription: Go work in a golaw repository.\n---\n"
	hundred := header + strings.Repeat("x\n", 100-lineCount([]byte(header)))
	for _, text := range []string{
		hundred,
		header + strings.Repeat("x", 10000-len(header)),
	} {
		if reason := inspectSkill("agents/SKILL.md", []byte(text)); reason != "" {
			t.Fatal(reason)
		}
	}
	for _, text := range []string{
		hundred + "x\n",
		header + strings.Repeat("x", 10001-len(header)),
		strings.Replace(header, "name: golaw", "name: other", 1),
		strings.Replace(header, "Go work in a golaw repository.", "", 1),
		header + "\u2014",
	} {
		if inspectSkill("agents/SKILL.md", []byte(text)) == "" {
			t.Fatal("invalid skill accepted")
		}
	}
	if lineCount(nil) != 0 || lineCount([]byte("x")) != 1 {
		t.Fatal("unterminated line counting is incorrect")
	}
}

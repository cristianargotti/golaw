package cicheck_test

import (
	"bytes"
	"strings"
	"testing"
)

func m6SkillSentences() []string {
	return []string{
		"Run golaw lint --surface before finishing when you changed a non-Go file.",
		"Move an inline script into a file, and export the code a test needs instead of evaluating it.",
	}
}

func TestM6SkillCarriesTheSurfaceSentences(t *testing.T) {
	t.Parallel()
	source := kitBytes(t, "agents/SKILL.md")
	copied := kitBytes(t, "agents/"+agentID(t)+"/skills/golaw/SKILL.md")
	if !bytes.Equal(source, copied) {
		t.Fatal("the agent copy of the skill differs from agents/SKILL.md")
	}
	for _, sentence := range m6SkillSentences() {
		if !strings.Contains(string(source), sentence) {
			t.Errorf("the skill lacks %q", sentence)
		}
		if strings.Contains(strings.Replace(string(source), sentence, "", 1), sentence) {
			t.Errorf("the skill repeats %q", sentence)
		}
	}
	if lineCount(source) > 100 || len(source) > 10000 {
		t.Errorf("the skill holds %d lines and %d bytes, over the A12 budget", lineCount(source), len(source))
	}
}

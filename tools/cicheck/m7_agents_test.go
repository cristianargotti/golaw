package cicheck_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

const (
	m7AgentHeader    = "| Agent | Read only denial | Regeneration feedback | Map at session start |"
	m7VersionsHeader = "| Agent | Version | Standing |"
)

var errM7Agents = errors.New("docs/agent-contracts.md generators")

// m7Agents answers the agents of the Versions table in their order. The
// names come from the document because the self scan keeps agent names out
// of this directory; the table is the one place that lists every agent.
func m7Agents(text string) []string {
	var agents []string
	for _, row := range m6TableRows(text, m7VersionsHeader) {
		name, _, _ := strings.Cut(strings.TrimPrefix(row, "| "), " | ")
		if !slices.Contains(agents, name) {
			agents = append(agents, name)
		}
	}
	return agents
}

func m7AgentsFit(text string) error {
	agents := m7Agents(text)
	if len(agents) != 3 {
		return fmt.Errorf("%w: the Versions table names %d agents", errM7Agents, len(agents))
	}
	for _, statement := range []string{
		"## Generators",
		"The reason of a denial is the same text on every agent",
		agents[2] + " writes are captured and allowed",
		"the m7-denial and m7-regen rows",
		"no live record of M7 is claimed here",
	} {
		if !strings.Contains(text, statement) {
			return fmt.Errorf("%w: lacks %q", errM7Agents, statement)
		}
	}
	rows := m6TableRows(text, m7AgentHeader)
	if len(rows) != len(agents) {
		return fmt.Errorf("%w: %d agent rows", errM7Agents, len(rows))
	}
	for i, agent := range agents {
		if !strings.HasPrefix(rows[i], "| "+agent+" | ") {
			return fmt.Errorf("%w: row %d is %q", errM7Agents, i, rows[i])
		}
	}
	if !strings.Contains(rows[0], "SessionStart") || strings.Contains(rows[1], "SessionStart") || strings.Contains(rows[2], "SessionStart") {
		return fmt.Errorf("%w: only %s receives the map at session start", errM7Agents, agents[0])
	}
	return nil
}

func TestM7AgentContractsGenerators(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, "docs/agent-contracts.md"))
	agents := m7Agents(text)
	if len(agents) != 3 {
		t.Fatalf("the Versions table names %v, want three agents", agents)
	}
	second := "| " + agents[1] + " | apply_patch"
	rows := []struct {
		name, text string
		bad        bool
	}{
		{"repository", text, false},
		{"red-live-claimed", strings.Replace(text, "no live record of M7 is claimed here", "", 1), true},
		{"red-second-agent-session", strings.Replace(text, second, "| "+agents[1]+" | SessionStart apply_patch", 1), true},
		{"red-third-agent-row-dropped", strings.Replace(text, "\n| "+agents[2]+" | none at the pre event", "\nnone at the pre event", 1), true},
		{"near-history-appended", text + "\nAn older record stays historical.\n", false},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if err := m7AgentsFit(row.text); (err != nil) != row.bad {
				t.Fatalf("contracts error = %v, want error %v", err, row.bad)
			}
		})
	}
}

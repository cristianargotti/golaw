package doctor

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/codex"
)

func TestCodexVersionLinesCompareWithThePinnedGrammar(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		version, failure, state, code, detail string
	}{
		{"codex-cli " + codex.PatchGrammar, "", stateOK, codeVersionValidated, "the release the apply_patch grammar is pinned to"},
		{"codex-cli 0.164.0", "", stateUnknown, codeVersionUnvalidated, "pinned to " + codex.PatchGrammar + ", see docs/agent-contracts.md"},
		{"codex-cli " + codex.PatchGrammar + "-alpha.10.2", "", stateUnknown, codeVersionUnvalidated, "-alpha.10.2"},
		{"", "resolve codex: agent environment unavailable", stateUnknown, codeVersionUnavailable, "through the search path the hook observed"},
		{"  ", "", stateUnknown, codeVersionUnavailable, "printed no version"},
	} {
		facts := installedFacts(agentCodex, "user")
		facts.Version, facts.VersionError = tc.version, tc.failure
		lines := codexVersionLines(facts)
		if len(lines) != 1 || lines[0].ID != idVersion || lines[0].Key != "codex/user" || lines[0].State != tc.state || lines[0].Code != tc.code || !strings.Contains(lines[0].Detail, tc.detail) {
			t.Errorf("%q %q: %+v", tc.version, tc.failure, lines)
		}
	}
	if codexVersionLines(installedFacts(agentCursor, "user")) != nil {
		t.Fatal("a Cursor report carries the Codex version")
	}
}

func TestCodexReportReadsTheVersionThroughTheObservedPath(t *testing.T) {
	t.Parallel()
	absent := filepath.Join(t.TempDir(), "receipt.json")
	input := Input{Agent: agentCodex, Scope: "user", ReceiptPath: absent}
	input.Environment = AgentEnvironment{Source: sourceAgent, Path: doubleNamed(t, agentCodex), PathExt: ".EXE"}
	got := lineWith(Codex(t.Context(), input, &enrollmentStore{}).Lines, idVersion)
	if got.State != stateOK || got.Code != codeVersionValidated || !strings.HasPrefix(got.Detail, "codex-cli "+codex.PatchGrammar) {
		t.Fatalf("version = %+v", got)
	}
	input.Environment = AgentEnvironment{}
	if got := lineWith(Codex(t.Context(), input, &enrollmentStore{}).Lines, idVersion); got.Code != codeVersionUnavailable || !strings.Contains(got.Detail, ErrEnvironment.Error()) {
		t.Fatalf("no observed environment = %+v", got)
	}
}

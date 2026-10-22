package doctor

import (
	"strings"
	"testing"
	"time"
)

// installedFacts is an agent installed and observed at a scope.
func installedFacts(agent, scope string) agentFacts {
	return agentFacts{
		Agent: agent, Scope: scope,
		Receipt:  receiptFact{State: receiptRead, Path: "receipt.json", Drift: []string{}},
		Events:   []eventFact{{Event: "preToolUse", Present: true, FailClosed: true}},
		Dispatch: Dispatch{State: DispatchObserved, SeenAt: time.Date(2026, time.October, 21, 9, 0, 0, 0, time.UTC), OS: "linux"},
	}
}

func TestInstalledLinesTellAbsentDriftedAndIntact(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, scope string
		receipt     receiptFact
		state, code string
		detail      string
	}{
		{"intact", "user", receiptFact{State: receiptRead, Path: "receipt.json"}, stateOK, codeInstalled, "every artifact, entry and block of receipt.json matches"},
		{"absent", "user", receiptFact{State: receiptAbsent}, stateUnknown, codeNotInstalled, "not installed for this scope: run golaw agent install --agent codex to use codex"},
		{"absent project", "project", receiptFact{State: receiptAbsent}, stateUnknown, codeNotInstalled, "run golaw agent install --agent codex --scope project to use codex"},
		{"unreadable", "user", receiptFact{State: receiptInvalid, Error: "receipt identity"}, stateRed, codeInstalledDrift, "the receipt cannot be read (receipt identity)"},
		{"drifted", "user", receiptFact{State: receiptRead, Drift: []string{"the artifact a", "the marked block in b"}}, stateRed, codeInstalledDrift, "the artifact a; the marked block in b; run golaw"},
	} {
		facts := installedFacts(agentCodex, tc.scope)
		facts.Receipt = tc.receipt
		line := installedLines(facts)[0]
		if line.ID != idInstalled || line.Key != "codex/"+tc.scope || line.State != tc.state || line.Code != tc.code || !strings.Contains(line.Detail, tc.detail) {
			t.Errorf("%s: %+v", tc.name, line)
		}
	}
}

func TestTrustedLinesNameWhatTheHostDecides(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		agent, scope, state, code, detail string
	}{
		{agentCursor, "user", stateOK, codeTrusted, "without a trust step"},
		{agentCursor, "project", stateUnknown, codeTrustUnknown, "trusted workspace"},
		{agentClaude, "user", stateUnknown, codeTrustUnknown, "until a live record exists"},
		{agentCodex, "user", stateUnknown, codeTrustUnknown, "trust is not read"},
	} {
		line := trustedLines(installedFacts(tc.agent, tc.scope))[0]
		if line.ID != idTrusted || line.State != tc.state || line.Code != tc.code || !strings.Contains(line.Detail, tc.detail) {
			t.Errorf("%s/%s: %+v", tc.agent, tc.scope, line)
		}
	}
}

func TestFailModeLinesNeverCallCodexClosed(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, agent string
		event       eventFact
		state, code string
		detail      string
	}{
		{"cursor", agentCursor, eventFact{Event: "preToolUse", Present: true, FailClosed: true}, stateOK, codeFailClosed, "failClosed"},
		{"claude", agentClaude, eventFact{Event: "PreToolUse", Present: true, FailClosed: true}, stateOK, codeFailClosed, "onFailure block"},
		{"codex", agentCodex, eventFact{Event: "PreToolUse", Present: true, FailClosed: true}, stateUnknown, codeHostFailsOpen, "Codex runs the tool"},
		{"softer", agentCursor, eventFact{Event: "preToolUse", Present: true}, stateRed, codeHostFailsOpen, "does not block on failure"},
		{"absent", agentClaude, eventFact{Event: "PreToolUse"}, stateRed, codeHostFailsOpen, "run golaw agent install --agent claude"},
	} {
		facts := installedFacts(tc.agent, "user")
		facts.Events = []eventFact{tc.event}
		line := failModeLines(facts)[0]
		if line.ID != idFailMode || line.Key != tc.agent+"/"+tc.event.Event || line.State != tc.state || line.Code != tc.code || !strings.Contains(line.Detail, tc.detail) {
			t.Errorf("%s: %+v", tc.name, line)
		}
	}
	if lines := failModeLines(agentFacts{Agent: agentCodex}); len(lines) != 0 {
		t.Fatalf("an installation without events = %+v", lines)
	}
}

func TestAgentDispatchLinesTellCurrentEarlierAndNever(t *testing.T) {
	t.Parallel()
	for state, want := range map[string]Line{
		DispatchObserved:   {State: stateOK, Code: codeObserved, Detail: "the hook ran 2026-10-21T09:00:00Z on linux with the current definition"},
		DispatchStale:      {State: stateUnknown, Code: codeStale, Detail: "the hook ran 2026-10-21T09:00:00Z on linux with an earlier definition"},
		DispatchUnobserved: {State: stateUnknown, Code: codeUnobserved, Detail: "no hook process of codex ran for this repository under this home"},
	} {
		facts := installedFacts(agentCodex, "user")
		facts.Dispatch.State = state
		line := agentDispatchLines(facts)[0]
		if line.ID != idDispatch || line.Key != "codex/user" || line.State != want.State || line.Code != want.Code || line.Detail != want.Detail {
			t.Errorf("%s: %+v", state, line)
		}
	}
}

func TestStateLineIsProtectingOnlyWhenTheThreeHold(t *testing.T) {
	t.Parallel()
	ok, red, unknown := Line{State: stateOK}, Line{State: stateRed, Detail: "why"}, Line{State: stateUnknown, Detail: "why"}
	for _, tc := range []struct {
		name                         string
		installed, trusted, dispatch Line
		state, code                  string
	}{
		{"protecting", ok, ok, ok, stateOK, codeProtecting},
		{"not installed", unknown, ok, ok, stateUnknown, codeNotInstalled},
		{"drifted", red, ok, ok, stateRed, codeNotInstalled},
		{"untrusted", ok, red, ok, stateRed, codeInstalledUntrusted},
		{"trust unknown", ok, unknown, ok, stateUnknown, codeTrustUnknown},
		{"inactive", ok, ok, unknown, stateUnknown, codeTrustedInactive},
		{"untrusted and inactive", ok, red, unknown, stateRed, codeInstalledUntrusted},
	} {
		line := stateLine(installedFacts(agentCodex, "user"), tc.installed, tc.trusted, tc.dispatch)
		if line.ID != idState || line.Key != "codex/user" || line.State != tc.state || line.Code != tc.code {
			t.Errorf("%s: %+v", tc.name, line)
		}
	}
	cursor := stateLine(installedFacts(agentCursor, "user"), ok, ok, ok)
	if cursor.State != stateUnknown || cursor.Code != codePreviewActive || !strings.Contains(cursor.Detail, "writes are captured, not enforced") {
		t.Fatalf("an observed Cursor hook = %+v", cursor)
	}
}

func TestSummarySaysTheStateInTheConfiguredLanguage(t *testing.T) {
	t.Parallel()
	facts := installedFacts(agentCodex, "project")
	for _, tc := range []struct {
		language, code, want string
	}{
		{"", codeProtecting, "golaw: o hook de codex está instalado, é confiável e já rodou neste repositório."},
		{"pt-BR", codeNotInstalled, "golaw: a instalação de codex está ausente ou foi alterada; rode golaw agent install --agent codex --scope project."},
		{"pt-BR", codeInstalledUntrusted, "aprove-o no agente (no Codex, em /hooks)"},
		{"pt-BR", codeTrustedInactive, "ainda não rodou neste repositório"},
		{"pt-BR", codeTrustUnknown, "veja docs/agent-contracts.md"},
		{"en", codeNotInstalled, "golaw: the codex installation is absent or changed; run golaw agent install --agent codex --scope project."},
		{"en", codeTrustUnknown, "whether the agent runs it depends on a trust doctor cannot read"},
	} {
		if got := summary(tc.language, facts, Line{Code: tc.code}); !strings.Contains(got, tc.want) {
			t.Errorf("%s %s: %q", tc.language, tc.code, got)
		}
	}
	if got := summary("pt-BR", facts, Line{Code: "unknown-code"}); got != "" {
		t.Fatalf("a code outside the table = %q", got)
	}
	absent := "golaw: codex não está instalado neste escopo; para usá-lo, rode golaw agent install --agent codex --scope project."
	if got := summary("pt-BR", facts, Line{State: stateUnknown, Code: codeNotInstalled}); got != absent {
		t.Fatalf("an agent never installed = %q", got)
	}
	if got := summary("pt-BR", facts, Line{State: stateUnknown, Code: codePreviewActive}); !strings.Contains(got, "capturadas, ainda não bloqueadas") {
		t.Fatalf("the Cursor preview = %q", got)
	}
}

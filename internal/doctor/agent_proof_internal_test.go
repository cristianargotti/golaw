package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// agentFixture is the input.json of an agent diagnostic: the facts of the
// agent under "agent" and the three lines of the state under "state", each
// patched over its baseline, so a fixture of one family leaves every other
// family at its baseline and two families never yield one code together.
type agentFixture struct {
	Agent agentFacts `json:"agent"`
	State struct {
		Installed Line `json:"installed"`
		Trusted   Line `json:"trusted"`
		Dispatch  Line `json:"dispatch"`
	} `json:"state"`
}

// agentProofBaseline is Cursor installed at the user scope, failing closed
// and observed: every family of the frame at ok.
func agentProofBaseline() agentFixture {
	var fixture agentFixture
	fixture.Agent = agentFacts{
		Agent: agentCursor, Scope: "user",
		Receipt:  receiptFact{State: receiptRead, Path: "receipt.json", Drift: []string{}},
		Events:   []eventFact{{Event: "preToolUse", Present: true, FailClosed: true}},
		Dispatch: Dispatch{State: DispatchObserved, OS: "linux"},
	}
	fixture.State.Installed = Line{ID: idInstalled, State: stateOK, Code: codeInstalled}
	fixture.State.Trusted = Line{ID: idTrusted, State: stateOK, Code: codeTrusted}
	fixture.State.Dispatch = Line{ID: idDispatch, State: stateOK, Code: codeObserved}
	return fixture
}

// agentProofCase binds one code of one line to the family that yields it;
// its proof id is the line id and the code joined.
type agentProofCase struct {
	line, code string
	run        func(agentFixture) []Line
}

func agentProofCases() []agentProofCase {
	facts := func(run func(agentFacts) []Line) func(agentFixture) []Line {
		return func(f agentFixture) []Line { return run(f.Agent) }
	}
	state := func(f agentFixture) []Line {
		return []Line{stateLine(f.Agent, f.State.Installed, f.State.Trusted, f.State.Dispatch)}
	}
	return []agentProofCase{
		{idInstalled, codeNotInstalled, facts(installedLines)},
		{idInstalled, codeInstalledDrift, facts(installedLines)},
		{idTrusted, codeTrustUnknown, facts(trustedLines)},
		{idFailMode, codeHostFailsOpen, facts(failModeLines)},
		{idDispatch, codeStale, facts(agentDispatchLines)},
		{idDispatch, codeUnobserved, facts(agentDispatchLines)},
		{idState, codeNotInstalled, state},
		{idState, codeInstalledUntrusted, state},
		{idState, codeTrustUnknown, state},
		{idState, codeTrustedInactive, state},
		{idState, codePreviewActive, state},
		{idPreview, codeWritesNotEnforced, facts(previewLines)},
		{idCapture, codeCaptureRecorded, facts(captureLines)},
		{idCapture, codeCaptureAbsent, facts(captureLines)},
		{idDuplicate, codeDuplicateHook, facts(duplicateLines)},
		{idDuplicate, codeSkillDuplicate, facts(duplicateLines)},
		{idCompanion, codeCompanionFound, facts(companionLines)},
	}
}

// agentDiagnosticCheck proves one code of one agent line both ways: it
// reports that code alone, so the other codes of the same line keep their
// own proof.
type agentDiagnosticCheck struct{ agentProofCase }

func (c agentDiagnosticCheck) Spec() model.Spec {
	id := c.line + "-" + c.code
	return model.Spec{ID: id, Rule: "rules/5.6@1", Scope: model.ScopeTree, Owner: model.OwnerCore, Proof: id}
}

func (c agentDiagnosticCheck) Run(_ context.Context, in model.Input) (model.Result, error) {
	data, err := in.Snapshot.Read("input.json")
	if err != nil {
		return model.Result{}, fmt.Errorf("read agent diagnostic fixture: %w", err)
	}
	fixture := agentProofBaseline()
	if err := json.Unmarshal(data, &fixture); err != nil {
		return model.Result{}, fmt.Errorf("decode agent diagnostic fixture: %w", err)
	}
	result := model.Result{Status: model.Measured(), Scope: c.line}
	for _, line := range c.run(fixture) {
		if line.ID == c.line && line.Code == c.code {
			result.Findings = append(result.Findings, model.Finding{Code: c.code, Message: line.Detail, Severity: model.SeverityNote})
		}
	}
	return result, nil
}

func registerAgentProof(t *testing.T, builder *engine.Builder) {
	t.Helper()
	for _, item := range agentProofCases() {
		if err := builder.Register(agentDiagnosticCheck{item}); err != nil {
			t.Fatal(err)
		}
	}
}

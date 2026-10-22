package doctor

import (
	"fmt"
	"strings"
)

// The lines that tell installed, trusted and active apart (D20).
const (
	idInstalled = "doctor.installed"
	idTrusted   = "doctor.trusted"
	idState     = "doctor.state"
	idFailMode  = "doctor.fail-mode"
	idDispatch  = "doctor.dispatch"
)

// The codes of those lines.
const (
	codeInstalled          = "installed"
	codeNotInstalled       = "not-installed"
	codeInstalledDrift     = "installed-drift"
	codeTrusted            = "trusted"
	codeTrustUnknown       = "trust-unknown"
	codeObserved           = "observed"
	codeStale              = "stale"
	codeUnobserved         = "unobserved"
	codeProtecting         = "protecting"
	codeInstalledUntrusted = "installed-untrusted"
	codeTrustedInactive    = "trusted-inactive"
	codeFailClosed         = "fail-closed"
	codeHostFailsOpen      = "host-fails-open"
	codePreviewActive      = "preview-active"
)

// agentAssessment is one line family of an agent report, a function of the
// facts alone.
type agentAssessment struct {
	id  string
	run func(agentFacts) []Line
}

// agentAssessments are the families of every agent report in print order;
// a family that does not apply to an agent answers no line for it.
func agentAssessments() []agentAssessment {
	return []agentAssessment{
		{idInstalled, installedLines},
		{idTrusted, trustedLines},
		{idFailMode, failModeLines},
		{idPreview, previewLines},
		{idCapture, captureLines},
		{idDuplicate, duplicateLines},
		{idCompanion, companionLines},
	}
}

// scopeKey is the key of a line about the agent and its scope.
func (f agentFacts) scopeKey() string {
	return f.Agent + "/" + f.Scope
}

// installCommand is the command that installs the agent at its scope, the
// fix a line names.
func (f agentFacts) installCommand() string {
	command := "golaw agent install --agent " + f.Agent
	if f.Scope == scopeProject {
		command += " --scope " + scopeProject
	}
	return command
}

// installedLines say whether the receipt exists and every artifact, entry
// and block it lists is present and matches.
func installedLines(f agentFacts) []Line {
	line := Line{
		ID: idInstalled, Key: f.scopeKey(), State: stateOK, Code: codeInstalled,
		Detail: "every artifact, entry and block of " + f.Receipt.Path + " matches",
	}
	switch {
	case f.Receipt.State == receiptAbsent:
		line.State, line.Code = stateUnknown, codeNotInstalled
		line.Detail = "not installed for this scope: run " + f.installCommand() + " to use " + f.Agent
	case f.Receipt.State != receiptRead:
		line.State, line.Code = stateRed, codeInstalledDrift
		line.Detail = "the receipt cannot be read (" + f.Receipt.Error + "): run " + f.installCommand()
	case len(f.Receipt.Drift) > 0:
		line.State, line.Code = stateRed, codeInstalledDrift
		line.Detail = "absent or changed since the install: " + strings.Join(f.Receipt.Drift, "; ") + "; run " + f.installCommand()
	}
	return []Line{line}
}

// trustedLines say whether the host will run the hook: Cursor runs a user
// hook without a trust step and a project hook only in a trusted workspace
// doctor cannot read; whether Claude Code runs a plugin hook before the
// workspace is trusted is unknown until a live record exists (section 8.3).
func trustedLines(f agentFacts) []Line {
	line := Line{ID: idTrusted, Key: f.scopeKey(), State: stateUnknown, Code: codeTrustUnknown}
	switch {
	case f.Agent == agentCursor && f.Scope != scopeProject:
		line.State, line.Code, line.Detail = stateOK, codeTrusted, "Cursor runs user hooks without a trust step"
	case f.Agent == agentCursor:
		line.Detail = "Cursor runs project hooks only in a trusted workspace, which doctor cannot read"
	case f.Agent == agentClaude:
		line.Detail = "whether Claude Code runs a plugin hook before the workspace is trusted is unknown until a live record exists"
	default:
		line.Detail = "the Codex trust is not read"
	}
	return []Line{line}
}

// failModeLines say, per event, whether the host blocks the tool when the
// hook fails: Claude's onFailure block and Cursor's failClosed do, and
// Codex never does, so its line is the stated limit, unknown and never ok,
// while a handler the person can fix is red.
func failModeLines(f agentFacts) []Line {
	lines := make([]Line, 0, len(f.Events))
	for _, event := range f.Events {
		line := Line{ID: idFailMode, Key: f.Agent + "/" + event.Event, State: stateOK, Code: codeFailClosed, Detail: failClosedDetail(f.Agent)}
		switch {
		case f.Agent == agentCodex:
			line.State, line.Code = stateUnknown, codeHostFailsOpen
			line.Detail = "Codex runs the tool when its hook fails, times out or is skipped; the internal deadline, the rules file, the post feedback, the staged scan and the pipeline are the mitigations"
		case !event.Present || !event.FailClosed:
			line.State, line.Code = stateRed, codeHostFailsOpen
			line.Detail = "the handler is absent or does not block on failure; run " + f.installCommand()
		}
		lines = append(lines, line)
	}
	return lines
}

// failClosedDetail names the setting that makes the host block.
func failClosedDetail(agent string) string {
	if agent == agentCursor {
		return "failClosed: a crash, a timeout or no output blocks the action"
	}
	return "onFailure block: a crash or a timeout blocks the tool"
}

// agentDispatchLines are the last dispatch evidence of the agent, the final
// line of its report (section 5.6): the host ran the hook for this
// repository with the current definition, an earlier one, or never.
func agentDispatchLines(f agentFacts) []Line {
	line := Line{
		ID: idDispatch, Key: f.scopeKey(), State: stateUnknown, Code: codeUnobserved,
		Detail: "no hook process of " + f.Agent + " ran for this repository under this home",
	}
	switch f.Dispatch.State {
	case DispatchObserved:
		line.State, line.Code = stateOK, codeObserved
		line.Detail = fmt.Sprintf("the hook ran %s on %s with the current definition", stamp(f.Dispatch.SeenAt), f.Dispatch.OS)
	case DispatchStale:
		line.Code = codeStale
		line.Detail = fmt.Sprintf("the hook ran %s on %s with an earlier definition", stamp(f.Dispatch.SeenAt), f.Dispatch.OS)
	}
	return []Line{line}
}

// stateLine is the one line that tells installed, trusted and active apart:
// protecting only when the three hold, so an inert hook is never reported
// as protecting, and never for Cursor, whose writes are captured, not
// enforced.
func stateLine(f agentFacts, installed, trusted, dispatch Line) Line {
	line := Line{ID: idState, Key: f.scopeKey(), State: stateOK, Code: codeProtecting, Detail: "installed, trusted and active"}
	switch {
	case installed.State != stateOK:
		line.State, line.Code, line.Detail = installed.State, codeNotInstalled, installed.Detail
	case trusted.State == stateRed:
		line.State, line.Code = stateRed, codeInstalledUntrusted
		line.Detail = "installed, and the host will not run the hook: " + trusted.Detail
	case trusted.State != stateOK:
		line.State, line.Code = stateUnknown, codeTrustUnknown
		line.Detail = "installed, and whether the host runs the hook is unknown: " + trusted.Detail
	case dispatch.State != stateOK:
		line.State, line.Code = stateUnknown, codeTrustedInactive
		line.Detail = "installed and trusted, and no hook process ran for this repository with the current definition yet"
	case f.Agent == agentCursor:
		line.State, line.Code = stateUnknown, codePreviewActive
		line.Detail = "shell and read guards ran; writes are captured, not enforced"
	}
	return line
}

// frame is every family of the agent and the state line computed from its
// installed and trusted lines and the dispatch line the caller prints last.
func frame(f agentFacts, dispatch Line) []Line {
	var lines []Line
	for _, item := range agentAssessments() {
		lines = append(lines, item.run(f)...)
	}
	return append(lines, stateLine(f, lineWith(lines, idInstalled), lineWith(lines, idTrusted), dispatch))
}

// lineWith is the first line with an id, the zero line when none.
func lineWith(lines []Line, id string) Line {
	for _, line := range lines {
		if line.ID == id {
			return line
		}
	}
	return Line{}
}

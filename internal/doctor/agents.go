package doctor

import "context"

// Claude is the report of the Claude Code agent: the evidence of its plugin,
// its settings and its hook process, assessed, with the lines that tell
// installed, trusted and active apart before its final dispatch line.
func Claude(ctx context.Context, input Input, enrollment EnrollmentReader) Report {
	return claudeReport(input, Collect(ctx, input, enrollment, NativeRunner{}))
}

// Codex is the report of the Codex CLI agent.
func Codex(ctx context.Context, input Input, _ EnrollmentReader) Report {
	facts := collectAgent(input)
	facts.Version, facts.VersionError = readVersion(ctx, agentVersion{command: agentCodex}, input.Environment)
	facts.Trust = codexTrust(ctx, input, appServer{version: RunningMetadata().Version})
	facts.Chain = collectChain(input)
	return agentReport(input.Language, facts)
}

// Cursor is the report of the Cursor agent, a compatibility preview.
func Cursor(_ context.Context, input Input, _ EnrollmentReader) Report {
	return agentReport(input.Language, collectAgent(input))
}

// claudeReport keeps every line of the M1 assessment in its order and puts
// the frame of the agent before the dispatch line, which stays the last.
func claudeReport(input Input, snapshot Snapshot) Report {
	facts := claudeFacts(input, snapshot)
	report := Report{Schema: 1, Lines: []Line{}}
	for _, item := range assessments() {
		lines := item.run(snapshot)
		if item.id == idDispatch {
			report.Lines = append(report.Lines, frame(facts, lineWith(lines, idDispatch))...)
		}
		report.Lines = append(report.Lines, lines...)
	}
	report.Summary = summary(input.Language, facts, lineWith(report.Lines, idState))
	return report
}

// agentReport is the report of an agent whose hooks live in a hooks file:
// its frame, then the dispatch line last.
func agentReport(language string, facts agentFacts) Report {
	dispatch := agentDispatchLines(facts)[0]
	lines := append(frame(facts, dispatch), dispatch)
	return Report{Schema: 1, Lines: lines, Summary: summary(language, facts, lineWith(lines, idState))}
}

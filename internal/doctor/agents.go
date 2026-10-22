package doctor

import "context"

// Claude is the report of the Claude Code agent: the evidence of its plugin,
// its settings and its hook process, assessed.
func Claude(ctx context.Context, input Input, enrollment EnrollmentReader) Report {
	return Assess(Collect(ctx, input, enrollment, NativeRunner{}))
}

// Codex is the report of the Codex CLI agent.
func Codex(_ context.Context, input Input, _ EnrollmentReader) Report {
	return notBuilt(input.Agent)
}

// Cursor is the report of the Cursor agent, a compatibility preview.
func Cursor(_ context.Context, input Input, _ EnrollmentReader) Report {
	return notBuilt(input.Agent)
}

// notBuilt is the report of an agent whose diagnostics are not built yet:
// unknown, never ok, so an inert hook is never reported as protecting.
func notBuilt(agent string) Report {
	return Report{Schema: 1, Lines: []Line{{
		ID: "doctor.agent", Key: agent, State: stateUnknown,
		Code: "agent-doctor-not-built", Detail: "the " + agent + " diagnostics are not built yet",
	}}}
}

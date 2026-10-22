package doctor

import (
	"context"
	"strings"

	"github.com/cristianargotti/golaw/internal/hook/codex"
)

// The version line of the Codex report: the installed Codex CLI against the
// release whose apply_patch grammar the decoder reads (D10).
const (
	idVersion              = "doctor.version"
	codeVersionValidated   = "agent-version-validated"
	codeVersionUnvalidated = "agent-version-unvalidated"
)

// agentVersion asks the agent of a command for its version, resolved
// through the search path the hook observed, without a shell.
type agentVersion struct{ command string }

// Version reads the first line the agent prints for --version.
func (v agentVersion) Version(ctx context.Context, environment AgentEnvironment) (string, error) {
	return runVersion(ctx, v.command, environment)
}

// readVersion is the version a runner reads, or why it read none.
func readVersion(ctx context.Context, runner VersionRunner, environment AgentEnvironment) (string, string) {
	version, err := runner.Version(ctx, environment)
	if err != nil {
		return "", err.Error()
	}
	return version, ""
}

// codexVersionLines compare the version the Codex CLI prints with the
// pinned grammar: equal is validated, another release is unvalidated, and
// a version doctor could not read is unavailable, never validated.
func codexVersionLines(f agentFacts) []Line {
	if f.Agent != agentCodex {
		return nil
	}
	fields := strings.Fields(f.Version)
	line := Line{
		ID: idVersion, Key: f.scopeKey(), State: stateOK, Code: codeVersionValidated,
		Detail: f.Version + ", the release the apply_patch grammar is pinned to",
	}
	switch {
	case f.VersionError != "":
		line.State, line.Code = stateUnknown, codeVersionUnavailable
		line.Detail = "codex --version through the search path the hook observed: " + f.VersionError
	case len(fields) == 0:
		line.State, line.Code = stateUnknown, codeVersionUnavailable
		line.Detail = "codex --version printed no version"
	case fields[len(fields)-1] != codex.PatchGrammar:
		line.State, line.Code = stateUnknown, codeVersionUnvalidated
		line.Detail = f.Version + "; the apply_patch grammar is pinned to " + codex.PatchGrammar + ", see docs/agent-contracts.md"
	}
	return []Line{line}
}

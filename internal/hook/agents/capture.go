package agents

import "path/filepath"

// capturesDir is the segment of the per agent state that holds the capture
// records doctor reads.
const capturesDir = "captures"

// CapturePath is where a hook process records the key names of the payloads
// its adapter could not judge yet (A4), keyed by the project identity, for
// doctor to read.
func CapturePath(stateDir, agent, identity string) string {
	return filepath.Join(stateDir, agent, capturesDir, identity+".json")
}

// HookAsset is the embedded asset of an agent's hook definition, the one an
// observation and a capture bind to: the hooks file of the Claude plugin,
// and the hooks file the installer merges for an agent without a plugin.
func HookAsset(id string) string {
	if id == "claude" {
		return "agents/claude/hooks/hooks.json"
	}
	return "agents/" + id + "/hooks.json"
}

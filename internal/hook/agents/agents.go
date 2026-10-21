// Package agents lists the agents the binary answers: for each one its id,
// the wire adapter of its hook, the planners of its installation and of
// its removal, its doctor report and the directory its host keeps its
// settings in. The command tree reads this list, so no agent is named
// outside the hook, install and doctor scopes the public boundary allows.
package agents

import (
	"context"
	"path/filepath"
	"slices"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/claude"
	"github.com/cristianargotti/golaw/internal/hook/codex"
	"github.com/cristianargotti/golaw/internal/hook/cursor"
	"github.com/cristianargotti/golaw/internal/install"
)

// The segments of the per agent state the installer and the hook share.
const (
	receiptFile     = "receipt.json"
	observationsDir = "observations"
	scopeProject    = "project"
)

// Agent is one agent the binary answers.
type Agent struct {
	ID        string                                                                     // the lower case id of hook <agent> and of --agent
	Adapter   hook.Adapter                                                               // the decoder and the encoder of its hook events
	Plan      func(install.Options) (install.Plan, error)                                // the planner of its installation
	Uninstall func(receiptPath string) (install.Plan, error)                             // the planner of its removal, from its receipt
	Doctor    func(context.Context, doctor.Input, doctor.EnrollmentReader) doctor.Report // its doctor report
	ConfigDir func(env func(string) string, home string) string                          // where its host reads settings and plugins
}

// All lists the agents of this version in id order.
func All() []Agent {
	return []Agent{{
		ID:        "claude",
		Adapter:   hook.Adapter{Parse: claude.Parse, Encode: claude.Encode, Oversized: claude.Oversized},
		Plan:      install.PlanClaude,
		Uninstall: install.PlanUninstall,
		Doctor:    doctor.Claude,
		ConfigDir: claudeConfigDir,
	}, {
		ID:        "codex",
		Adapter:   hook.Adapter{Parse: codex.Parse, Encode: codex.Encode, Oversized: codex.Oversized},
		Plan:      install.PlanCodex,
		Uninstall: install.UninstallCodex,
		Doctor:    doctor.Codex,
		ConfigDir: codexConfigDir,
	}, {
		ID:        "cursor",
		Adapter:   hook.Adapter{Parse: cursor.Parse, Encode: cursor.Encode, Oversized: cursor.Oversized},
		Plan:      install.PlanCursor,
		Uninstall: install.UninstallCursor,
		Doctor:    doctor.Cursor,
		ConfigDir: cursorConfigDir,
	}}
}

// claudeConfigDir honors the directory override of the host and falls back
// to the dot directory under the home.
func claudeConfigDir(env func(string) string, home string) string {
	if dir := env("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(home, ".claude")
}

// codexConfigDir is the Codex home: the directory its override names, or
// the dot directory under the home.
func codexConfigDir(env func(string) string, home string) string {
	if dir := env("CODEX_HOME"); dir != "" {
		return dir
	}
	return filepath.Join(home, ".codex")
}

// cursorConfigDir is where Cursor reads the user hooks file; it documents
// no override.
func cursorConfigDir(_ func(string) string, home string) string {
	return filepath.Join(home, ".cursor")
}

// IDs lists the agent ids, sorted.
func IDs() []string {
	all := All()
	ids := make([]string, 0, len(all))
	for _, a := range all {
		ids = append(ids, a.ID)
	}
	slices.Sort(ids)
	return ids
}

// Lookup finds an agent by id.
func Lookup(id string) (Agent, bool) {
	for _, a := range All() {
		if a.ID == id {
			return a, true
		}
	}
	return Agent{}, false
}

// ReceiptPath is where the installation receipt of an agent lives under the
// state directory, as the installer lays it out: <state>/<agent>/<scope>
// for the user scope, with the project identity appended for a project.
func ReceiptPath(stateDir, agent, scope, identity string) string {
	store := filepath.Join(stateDir, agent, scope)
	if scope == scopeProject {
		store = filepath.Join(store, identity)
	}
	return filepath.Join(store, receiptFile)
}

// ObservationPath is where a hook process records the environment its agent
// gave it, keyed by the project identity, for doctor to read.
func ObservationPath(stateDir, agent, identity string) string {
	return filepath.Join(stateDir, agent, observationsDir, identity+".json")
}

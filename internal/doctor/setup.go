package doctor

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/setup"
)

const (
	idSetupReceipt = "doctor.setup-receipt"
	idSetupHooks   = "doctor.setup-hooks"
	idSetupRouting = "doctor.setup-routing"
	idSetupTools   = "doctor.setup-tools"
	setupPresent   = "present"
	setupAbsent    = "absent"
	setupInvalid   = "invalid"
	setupDrift     = "drift"
	setupMissing   = "missing"
)

// ToolStatus keeps probe failures distinct from an executable absent from PATH.
type ToolStatus struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version"`
	Error   string `json:"error"`
}

// UnmarshalJSON prevents a new observation from retaining an earlier path or version.
func (s *ToolStatus) UnmarshalJSON(data []byte) error {
	type observation ToolStatus
	var value observation
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("decode setup tool observation: %w", err)
	}
	*s = ToolStatus(value)
	return nil
}

// SetupHook preserves the bytes observed separately from the receipt's claim.
type SetupHook struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Error  string `json:"error"`
}

// SetupFacts permits diagnostics to be replayed without probing the repository.
type SetupFacts struct {
	ReceiptPath        string        `json:"receipt_path"`
	Receipt            setup.Receipt `json:"receipt"`
	ReceiptState       string        `json:"receipt_state"`
	ReceiptError       string        `json:"receipt_error"`
	EffectiveHooksPath string        `json:"effective_hooks_path"`
	LocalHooksPath     string        `json:"local_hooks_path"`
	HooksPathError     string        `json:"hooks_path_error"`
	Hooks              []SetupHook   `json:"hooks"`
	Tools              []ToolStatus  `json:"tools"`
	Ladder             model.Ladder  `json:"ladder"`
}

type setupAssessment struct {
	id  string
	run func(SetupFacts) []Line
}

func setupAssessments() []setupAssessment {
	return []setupAssessment{
		{idSetupReceipt, setupReceiptLines},
		{idSetupHooks, setupHookLines},
		{idSetupRouting, setupRoutingLines},
		{idSetupTools, setupToolLines},
	}
}

// AssessSetup keeps optional local setup separate from production findings.
func AssessSetup(facts SetupFacts) []Line {
	assessments := setupAssessments()
	lines := make([]Line, 0, len(assessments))
	for _, item := range assessments {
		lines = append(lines, item.run(facts)...)
	}
	return lines
}

func setupReceiptLines(facts SetupFacts) []Line {
	line := Line{ID: idSetupReceipt, Key: facts.ReceiptPath, State: stateOK, Code: "setup-receipt-present"}
	switch facts.ReceiptState {
	case setupAbsent:
		line.State, line.Code = "not-armed", "setup-not-armed"
		line.Detail = "local hooks are optional; run golaw setup to arm them"
	case setupPresent:
		line.Detail = "setup receipt is present"
	default:
		line.State, line.Code = setupInvalid, "setup-receipt-invalid"
		line.Detail = facts.ReceiptError + "; inspect the receipt, then run golaw setup again"
	}
	return []Line{line}
}

func setupHookLines(facts SetupFacts) []Line {
	if facts.ReceiptState != setupPresent {
		return nil
	}
	lines := setupRungLines(facts.Ladder)
	for _, recorded := range facts.Receipt.Hooks {
		line := Line{ID: idSetupHooks, Key: recorded.Path, State: stateOK, Code: "setup-hook-matches"}
		index := slices.IndexFunc(facts.Hooks, func(hook SetupHook) bool { return hook.Path == recorded.Path })
		switch {
		case index < 0:
			line.State, line.Detail = setupMissing, "hook evidence is missing"
		case facts.Hooks[index].Error != "":
			line.State, line.Detail = stateUnknown, facts.Hooks[index].Error
			if line.Detail == setupMissing || strings.HasPrefix(line.Detail, setupMissing+": ") {
				line.State = setupMissing
			}
		case facts.Hooks[index].SHA256 != recorded.SHA256:
			line.State, line.Detail = setupDrift, "hook hash differs from the setup receipt"
		}
		if line.State != stateOK {
			line.Code = "setup-hook-drift"
			line.Detail += "; inspect the hook, then run golaw setup again"
		}
		lines = append(lines, line)
	}
	return lines
}

func setupRungLines(ladder model.Ladder) []Line {
	var lines []Line
	for _, rung := range []struct {
		name    string
		enabled bool
	}{
		{"pre-commit", ladder.PreCommit},
		{"commit-msg", ladder.CommitMsg},
		{"pre-push", ladder.PrePush},
	} {
		if !rung.enabled {
			lines = append(lines, Line{
				ID: idSetupHooks, Key: rung.name, State: "disabled", Code: "setup-rung-disabled",
				Detail: rung.name + " is disabled in golaw.yml; local rungs are optional",
			})
		}
	}
	return lines
}

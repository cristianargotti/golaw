package doctor

import (
	"encoding/json"
	"maps"
	"path/filepath"
	"slices"

	"github.com/cristianargotti/golaw/internal/install"
)

// The line of the companion gate of A17 in the hooks file of an agent,
// found by its neutral script name.
const (
	idCompanion         = "doctor.companion"
	codeCompanionAbsent = "companion-absent"
	codeCompanionFound  = "companion-found"
	companionScript     = "standards_gate.py"
	orderBefore         = "before"
	orderAfter          = "after"
	orderAlone          = "alone"
	hooksFile           = "hooks.json"
)

// companionFact is one handler of the companion gate: the event it
// answers, the hooks file and where it runs against golaw's handler of the
// same event, in file order.
type companionFact struct {
	Event string `json:"event"`
	Path  string `json:"path"`
	Order string `json:"order"`
}

// collectCompanions reads the hooks file of the agent, the one the receipt
// names or the one of the scope when nothing is installed, and finds every
// handler that runs the companion gate.
func collectCompanions(input Input, receipt install.Receipt) []companionFact {
	if input.Agent != agentCodex && input.Agent != agentCursor {
		return nil
	}
	hooksPath := receipt.HooksPath
	if hooksPath == "" {
		hooksPath = defaultHooksPath(input)
	}
	slots, err := readSlots(hooksPath, input.Agent == agentCursor)
	if err != nil {
		return nil
	}
	var out []companionFact
	for _, event := range slices.Sorted(maps.Keys(slots)) {
		golaw := ownedIndex(slots[event], receipt, event)
		for index, candidate := range slots[event] {
			if runsCompanion(candidate.Raw) {
				out = append(out, companionFact{Event: event, Path: hooksPath, Order: orderOf(index, golaw)})
			}
		}
	}
	return out
}

// defaultHooksPath is the hooks file of the scope (D12): the one in the
// host's directory at the user scope, the one in the repository's dot
// directory of the agent with --scope project.
func defaultHooksPath(input Input) string {
	if input.Scope == scopeProject {
		return filepath.Join(input.Root, "."+input.Agent, hooksFile)
	}
	return filepath.Join(input.ConfigDir, hooksFile)
}

// ownedIndex is the first slot of the event holding a handler the receipt
// owns, or -1.
func ownedIndex(slots []slot, receipt install.Receipt, event string) int {
	first := -1
	for _, entry := range receipt.Entries {
		if entry.Event != event {
			continue
		}
		if index := locate(slots, entry.Definition); index >= 0 && (first < 0 || index < first) {
			first = index
		}
	}
	return first
}

// orderOf places the companion against golaw's handler in file order.
func orderOf(index, golaw int) string {
	switch {
	case golaw < 0:
		return orderAlone
	case index < golaw:
		return orderBefore
	default:
		return orderAfter
	}
}

// runsCompanion reports whether a handler's command names the gate script
// as a word, the way the coexistence detection of A17 recognizes it; a lone
// echo or printf runs nothing.
func runsCompanion(raw json.RawMessage) bool {
	fields := fieldsOf(raw)
	for _, command := range []string{fields.Command, fields.CommandWindows} {
		words := commandWords(command, nil)
		if len(words) == 0 || slices.Contains([]string{"echo", "printf"}, baseName(words[0])) {
			continue
		}
		if slices.ContainsFunc(words, func(word string) bool { return baseName(word) == companionScript }) {
			return true
		}
	}
	return false
}

// companionLines name every handler of the companion gate and where it runs
// against golaw, or say there is none.
func companionLines(f agentFacts) []Line {
	if f.Agent != agentCodex && f.Agent != agentCursor {
		return nil
	}
	if len(f.Companions) == 0 {
		return []Line{{ID: idCompanion, Key: f.scopeKey(), State: stateOK, Code: codeCompanionAbsent, Detail: "no handler runs " + companionScript + " in the hooks file"}}
	}
	lines := make([]Line, 0, len(f.Companions))
	for _, found := range f.Companions {
		lines = append(lines, Line{
			ID: idCompanion, Key: f.scopeKey(), State: stateUnknown, Code: codeCompanionFound,
			Detail: companionScript + " answers " + found.Event + " in " + found.Path + " " + orderText(found.Order) +
				"; two gates judge the event, and their ownership of the rule families is the table of A17",
		})
	}
	return lines
}

// orderText says where the companion runs against golaw.
func orderText(order string) string {
	switch order {
	case orderBefore:
		return "before golaw in file order"
	case orderAfter:
		return "after golaw in file order"
	default:
		return "where golaw has no handler"
	}
}

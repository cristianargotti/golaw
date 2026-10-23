package doctor

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"runtime"
	"slices"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/install"
)

// The agents whose hooks live in a hooks file the person shares with other
// tools, and the scope whose installation needs a trust step of its own.
const (
	agentCodex   = "codex"
	agentCursor  = "cursor"
	scopeProject = "project"
)

// The states of the receipt of an agent and scope.
const (
	receiptRead    = "read"
	receiptAbsent  = "absent"
	receiptInvalid = "invalid"
)

// agentFacts is the evidence of one agent's installation for one scope,
// every file already read, so each line of its report is a function of it
// alone and the proof patches it as JSON.
type agentFacts struct {
	Agent    string      `json:"agent"`
	Scope    string      `json:"scope"`
	Receipt  receiptFact `json:"receipt"`
	Events   []eventFact `json:"events"`
	Dispatch Dispatch    `json:"dispatch"`

	Captures     []hook.Capture `json:"captures"`
	CaptureError string         `json:"capture_error"`

	Duplicates []duplicateFact `json:"duplicates"`
	Companions []companionFact `json:"companions"`

	Version      string `json:"version"`
	VersionError string `json:"version_error"`

	Binaries []binaryFact `json:"binaries"`
	Trust    trustFact    `json:"trust"`
	Chain    chainFact    `json:"chain"`
}

// receiptFact is the installation as its receipt records it: whether the
// receipt was read, and what it lists that is absent or differs now.
type receiptFact struct {
	State string   `json:"state"`
	Path  string   `json:"path"`
	Error string   `json:"error"`
	Drift []string `json:"drift"`
}

// eventFact is one host event the installation answers: whether its
// handler is in the hooks file and whether the host blocks the tool when
// the handler fails.
type eventFact struct {
	Event      string `json:"event"`
	Present    bool   `json:"present"`
	FailClosed bool   `json:"fail_closed"`
}

// collectAgent reads the evidence of an agent whose hooks live in a hooks
// file: the receipt, every artifact, entry and block it lists, the fail
// mode of every event it answers and the dispatch the binding observed.
func collectAgent(input Input) agentFacts {
	facts := agentFacts{
		Agent: input.Agent, Scope: input.Scope, Dispatch: input.Dispatch,
		Captures: input.Captures, CaptureError: input.CaptureError,
	}
	receipt, fact := readReceiptFact(input.ReceiptPath)
	facts.Receipt = fact
	if fact.State == receiptRead {
		slots, err := readSlots(receipt.HooksPath, input.Agent == agentCursor)
		facts.Receipt.Drift = receiptDrift(receipt, input.Agent, slots, err)
		facts.Events = eventFacts(receipt, slots)
		facts.Binaries = collectBinaries(receipt, slots, input.Agent, input.RunningBinary, runtime.GOOS)
	}
	facts.Duplicates = collectDuplicates(input, receipt.HooksPath)
	facts.Companions = collectCompanions(input, receipt)
	return facts
}

// readReceiptFact tells an absent receipt, which means nothing was
// installed, from one that cannot be read, which means the installation
// cannot be vouched for.
func readReceiptFact(path string) (install.Receipt, receiptFact) {
	fact := receiptFact{State: receiptRead, Path: path}
	receipt, err := install.ReadReceipt(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		fact.State = receiptAbsent
	case err != nil:
		fact.State, fact.Error = receiptInvalid, err.Error()
	}
	return receipt, fact
}

// receiptDrift names every part of the installation the receipt lists that
// is absent or differs: an artifact by its bytes, an entry by its compact
// definition anywhere in its event, a block by the hash of its span.
func receiptDrift(receipt install.Receipt, agent string, slots map[string][]slot, slotsErr error) []string {
	drift := []string{}
	if receipt.Agent != agent {
		drift = append(drift, "the receipt belongs to the agent "+quoted(receipt.Agent))
	}
	for _, artifact := range receipt.Artifacts {
		data, err := os.ReadFile(artifact.Path)
		if err != nil || drifted(receipt, artifact, string(data)) {
			drift = append(drift, "the artifact "+artifact.Path)
		}
	}
	for _, entry := range receipt.Entries {
		if slotsErr != nil || !entryMatches(slots[entry.Event], entry) {
			drift = append(drift, "the "+entry.Event+" handler in "+receipt.HooksPath)
		}
	}
	for _, block := range receipt.Blocks {
		if !blockMatches(block) {
			drift = append(drift, "the marked block in "+block.Path)
		}
	}
	return drift
}

// entryMatches reports whether an event of the hooks file holds the entry
// the receipt recorded: its compact definition under the same matcher, so
// a matcher changed to select nothing is drift whatever its trust.
func entryMatches(slots []slot, entry install.Entry) bool {
	index := locate(slots, entry.Definition)
	return index >= 0 && slots[index].Matcher == entry.Matcher
}

// eventFacts is the fail mode of every event the receipt's entries answer,
// read from the handler as the hooks file holds it now.
func eventFacts(receipt install.Receipt, slots map[string][]slot) []eventFact {
	byEvent := map[string]eventFact{}
	for _, entry := range receipt.Entries {
		fact := eventFact{Event: entry.Event}
		if index := locate(slots[entry.Event], entry.Definition); index >= 0 {
			fact.Present, fact.FailClosed = true, failsClosed(slots[entry.Event][index].Raw)
		}
		if previous, seen := byEvent[entry.Event]; seen {
			fact.Present = fact.Present && previous.Present
			fact.FailClosed = fact.FailClosed && previous.FailClosed
		}
		byEvent[entry.Event] = fact
	}
	out := make([]eventFact, 0, len(byEvent))
	for _, event := range slices.Sorted(maps.Keys(byEvent)) {
		out = append(out, byEvent[event])
	}
	return out
}

// claudeFacts reads the frame of the Claude Code agent from the evidence of
// M1: the receipt drifts where an artifact or a hook line is not ok, and
// the fail mode of every event comes from the plugin hooks file.
func claudeFacts(input Input, snapshot Snapshot) agentFacts {
	_, fact := readReceiptFact(input.ReceiptPath)
	if fact.State == receiptRead {
		for _, line := range slices.Concat(artifactLines(snapshot), hookLines(snapshot)) {
			if line.State != stateOK {
				fact.Drift = append(fact.Drift, line.ID+" "+line.Key+" "+line.Code)
			}
		}
	}
	return agentFacts{
		Agent: agentClaude, Scope: input.Scope, Receipt: fact,
		Events: claudeEvents(snapshot.Hooks), Dispatch: snapshot.Dispatch,
	}
}

// claudeEvents is the fail mode of every event of the plugin hooks file:
// closed when every handler of the event blocks on failure.
func claudeEvents(hooks string) []eventFact {
	parsed, err := install.ParseHooks([]byte(hooks))
	if err != nil {
		return nil
	}
	out := make([]eventFact, 0, len(parsed.Hooks))
	for _, event := range slices.Sorted(maps.Keys(parsed.Hooks)) {
		fact := eventFact{Event: event, Present: true, FailClosed: true}
		for _, group := range parsed.Hooks[event] {
			for _, handler := range group.Hooks {
				fact.FailClosed = fact.FailClosed && handler.OnFailure == "block"
			}
		}
		out = append(out, fact)
	}
	return out
}

// quoted names a value a line prints, an empty one included.
func quoted(value string) string {
	return `"` + value + `"`
}

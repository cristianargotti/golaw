// Package check runs the gates the pipeline fails on over the change of a
// worktree and keeps only what is red (golaw check), because an agent that
// runs each verification command apart rereads its whole context per turn.
package check

import (
	"fmt"
	"strings"
)

// Status is the answer of one gate.
type Status string

// The three answers a gate gives: it passed, it failed, or it does not
// apply to this change, with the reason.
const (
	Green   Status = "green"
	Red     Status = "red"
	Skipped Status = "skipped"
)

// MaxLines bounds the failures a red gate prints, so a broken build of
// many files never floods the context of the agent that reads it.
const MaxLines = 12

// Result is what one gate answered.
type Result struct {
	Gate     string   `json:"gate"`
	Status   Status   `json:"status"`
	Reason   string   `json:"reason,omitempty"`   // why a gate was skipped
	Failures []string `json:"failures,omitempty"` // the lines of a red gate, every one of them
}

// Pass is a green gate.
func Pass(gate string) Result { return Result{Gate: gate, Status: Green} }

// Fail is a red gate with its failure lines; a red gate without a line
// still names the gate.
func Fail(gate string, failures ...string) Result {
	return Result{Gate: gate, Status: Red, Failures: failures}
}

// Skip is a gate that does not apply to the change, with its reason.
func Skip(gate, reason string) Result { return Result{Gate: gate, Status: Skipped, Reason: reason} }

// Report is the answer of one golaw check run.
type Report struct {
	Base    string   `json:"base"`       // the ref the change is judged against, empty for the whole tree
	Whole   bool     `json:"whole_tree"` // no base resolved, so every file counts as changed
	Note    string   `json:"note,omitempty"`
	Results []Result `json:"gates"`
}

// Count answers how many gates gave the status.
func (r Report) Count(status Status) int {
	n := 0
	for _, result := range r.Results {
		if result.Status == status {
			n++
		}
	}
	return n
}

// Red reports whether any gate failed.
func (r Report) Red() bool { return r.Count(Red) > 0 }

// Lines is the text report: the count line, the reason a whole tree was
// judged, every skipped gate in one line, each red gate with at most
// MaxLines of its failures, and the green gates only when verbose.
func (r Report) Lines(verbose bool) []string {
	lines := []string{fmt.Sprintf("golaw check: %d gates, %d green, %d red, %d skipped (%s)",
		len(r.Results), r.Count(Green), r.Count(Red), r.Count(Skipped), r.scope())}
	if r.Note != "" {
		lines = append(lines, r.Note)
	}
	for _, result := range r.Results {
		switch result.Status {
		case Red:
			lines = append(lines, "red "+result.Gate+":")
			lines = append(lines, indented(result.Failures)...)
		case Skipped:
			lines = append(lines, "skipped "+result.Gate+": "+result.Reason)
		case Green:
			if verbose {
				lines = append(lines, "green "+result.Gate)
			}
		}
	}
	return lines
}

func (r Report) scope() string {
	if r.Whole {
		return "whole tree"
	}
	return "base " + r.Base
}

// indented keeps the first MaxLines failures and counts the rest.
func indented(failures []string) []string {
	shown := failures[:min(len(failures), MaxLines)]
	out := make([]string, 0, len(shown)+1)
	for _, line := range shown {
		out = append(out, "  "+line)
	}
	if rest := len(failures) - len(shown); rest > 0 {
		out = append(out, fmt.Sprintf("  ... and %d more", rest))
	}
	return out
}

// nonBlank answers the trimmed lines of text that hold something.
func nonBlank(text string) []string {
	var out []string
	for line := range strings.SplitSeq(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

package engine

import (
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
)

// Verdict is the final word of a run.
type Verdict int

// The three verdicts: Clean lets the operation through, Fail is the scan
// outcome with enforced findings, Deny is the hook outcome.
const (
	VerdictClean Verdict = iota + 1
	VerdictFail
	VerdictDeny
)

// String names the verdict for reports.
func (v Verdict) String() string {
	switch v {
	case VerdictClean:
		return "clean"
	case VerdictFail:
		return "fail"
	case VerdictDeny:
		return "deny"
	default:
		return "unknown"
	}
}

// Unmeasured names a required check that could not run.
type Unmeasured struct {
	Check  string
	Reason string
}

// Decision is what the hooks and the scan consume. Reason is the English
// text for the agent, identical across agents; the human channel message
// is rendered elsewhere in the configured language.
type Decision struct {
	Mode       model.Mode
	Verdict    Verdict
	Findings   []model.Finding // every finding, notes included, in order
	Unmeasured []Unmeasured    // the required checks left unmeasured, in registry order
	Unread     []string        // the files over the read bound a measured check left unread, sorted
	Reason     string          // the first enforced finding or the first unmeasured check, empty when clean
}

// Decide turns a run into its verdict: any enforced finding or unmeasured
// check denies in hook mode and fails elsewhere; notes never count.
func Decide(run *Run) Decision {
	d := Decision{Mode: run.Mode, Verdict: VerdictClean, Findings: run.Findings(), Unread: run.Unread()}
	unmeasured := run.Unmeasured()
	for i := range unmeasured {
		o := &unmeasured[i]
		d.Unmeasured = append(d.Unmeasured, Unmeasured{Check: o.Spec.ID, Reason: o.Result.Status.Reason})
	}
	for k := range d.Findings {
		f := &d.Findings[k]
		if f.Enforced() {
			d.Reason = findingReason(*f)
			break
		}
	}
	if d.Reason == "" && len(d.Unmeasured) > 0 {
		d.Reason = fmt.Sprintf("%s: not measured: %s", d.Unmeasured[0].Check, d.Unmeasured[0].Reason)
	}
	if d.Reason == "" {
		return d
	}
	if run.Mode == model.ModeHook {
		d.Verdict = VerdictDeny
	} else {
		d.Verdict = VerdictFail
	}
	return d
}

// findingReason renders one finding for the agent: check, message and the
// location when the finding has one.
func findingReason(f model.Finding) string {
	switch {
	case f.Path == "":
		return fmt.Sprintf("%s: %s", f.Check, f.Message)
	case f.Line == 0:
		return fmt.Sprintf("%s: %s (%s)", f.Check, f.Message, f.Path)
	default:
		return fmt.Sprintf("%s: %s (%s:%d)", f.Check, f.Message, f.Path, f.Line)
	}
}

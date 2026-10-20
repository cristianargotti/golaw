package rung

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/report"
	"github.com/cristianargotti/golaw/internal/stage"
)

// maxNamed bounds the paths one line names, as the notices of the agent
// hooks do; the rest are counted.
const maxNamed = 20

// judged is the outcome of a run of the registry: the code of section 4.4,
// the English lines of the text report, the files the read bound left
// unjudged and, when git is refused, the sentence of the human channel in
// the language of the policy, which names the rule through the phrase
// table.
func judged(name, language string, run *engine.Run) Outcome {
	s := settled(run)
	lines, err := reportLines(run, s.decision)
	if err != nil {
		return failed(name, language, err)
	}
	out := Outcome{Code: codeOf(s.decision), Lines: append(lines, s.lines(name)...)}
	if out.Code != codeClean {
		out.Lines = append(out.Lines, report.Human(language, s.decision))
	}
	return out
}

// settlement is a decision once the checks a rung leaves to the pipeline
// count as measured: the files over the read bound, and the checks whose
// comparison with the base could not be measured, each with its reason.
type settlement struct {
	decision engine.Decision
	unread   []string
	based    []string
}

// settled is the decision of a run once every check the read bound alone
// left unmeasured, and every check the comparison with the base left
// unmeasured, counts as measured, with the files it could not read and the
// checks it could not compare aside: a rung refuses only what it
// recognizes with certainty (the owner's zero friction goal of
// 2026-10-16), and a base that does not parse is no fault of the commit
// and none the commit can repair, so such a file or check is named and
// left to the pipeline, while scan keeps exit 2 for it (section 5.2).
func settled(run *engine.Run) settlement {
	var s settlement
	for i := range run.Outcomes {
		res := &run.Outcomes[i].Result
		if res.Status.Kind != model.StatusUnmeasured {
			continue
		}
		reason, compared := strings.CutPrefix(res.Status.Reason, stage.BaseReason)
		switch {
		case len(res.Unread) > 0 && strings.HasSuffix(res.Status.Reason, model.ErrTooLarge.Error()):
		case compared:
			s.based = append(s.based, fmt.Sprintf("%s (%s)", run.Outcomes[i].Spec.ID, reason))
		default:
			continue
		}
		res.Status = model.Measured()
		s.unread = append(s.unread, res.Unread...)
	}
	slices.Sort(s.unread)
	s.unread = slices.Compact(s.unread)
	s.decision = engine.Decide(run)
	return s
}

// lines name what the settlement leaves to the pipeline.
func (s settlement) lines(name string) []string {
	var out []string
	if len(s.unread) > 0 {
		out = append(out, unreadLine(name, s.unread))
	}
	if len(s.based) > 0 {
		out = append(out, prefix(name)+"not judged against a base it could not measure, the pipeline judges them: "+strings.Join(s.based, "; "))
	}
	return out
}

// unreadLine names the files a rung leaves to the pipeline because they
// are over the read bound, at most maxNamed and the rest counted.
func unreadLine(name string, unread []string) string {
	return prefix(name) + "over the read bound, not judged here: " + named(unread) + "; the pipeline judges them"
}

// named joins the paths of one line, at most maxNamed, so a commit of
// thousands of files still answers git with a line a person reads.
func named(paths []string) string {
	shown, rest := paths, ""
	if len(shown) > maxNamed {
		shown, rest = shown[:maxNamed], fmt.Sprintf(" and %d more", len(paths)-maxNamed)
	}
	return strings.Join(shown, ", ") + rest
}

// reportLines renders a run as the text report a scan prints, one entry
// per line: the findings, the checks left unmeasured and the verdict.
func reportLines(run *engine.Run, decision engine.Decision) ([]string, error) {
	var text bytes.Buffer
	if err := report.WriteText(&text, report.New(run, decision, report.Identity{})); err != nil {
		return nil, fmt.Errorf("render the report: %w", err)
	}
	return strings.Split(strings.TrimSuffix(text.String(), "\n"), "\n"), nil
}

// codeOf maps a decision to the code of section 4.4: a required check left
// unmeasured is a failure before any finding is a finding.
func codeOf(d engine.Decision) int {
	switch {
	case len(d.Unmeasured) > 0:
		return codeFailure
	case d.Verdict == engine.VerdictClean:
		return codeClean
	default:
		return codeFindings
	}
}

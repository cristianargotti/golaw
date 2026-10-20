package rung

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/report"
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
	decision, unread := settled(run)
	lines, err := reportLines(run, decision)
	if err != nil {
		return failed(name, language, err)
	}
	out := Outcome{Code: codeOf(decision), Lines: lines}
	if len(unread) > 0 {
		out.Lines = append(out.Lines, unreadLine(name, unread))
	}
	if out.Code != codeClean {
		out.Lines = append(out.Lines, report.Human(language, decision))
	}
	return out
}

// settled is the decision of a run once every check the read bound alone
// left unmeasured counts as measured, with the files it could not read
// aside: a rung refuses only what it recognizes with certainty (the
// owner's zero friction goal of 2026-10-16), so such a file is named and
// left to the pipeline, while scan keeps exit 2 for it (section 5.2).
func settled(run *engine.Run) (engine.Decision, []string) {
	var unread []string
	for i := range run.Outcomes {
		res := &run.Outcomes[i].Result
		if res.Status.Kind == model.StatusUnmeasured && len(res.Unread) > 0 && strings.HasSuffix(res.Status.Reason, model.ErrTooLarge.Error()) {
			res.Status = model.Measured()
			unread = append(unread, res.Unread...)
		}
	}
	slices.Sort(unread)
	return engine.Decide(run), slices.Compact(unread)
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

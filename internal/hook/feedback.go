package hook

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// maxFileLines bounds the feedback lines of one file (M6.6.3), so a file
// with many findings leaves room for the others and the rest is counted.
const maxFileLines = 20

// causeKilled is the notice of a file whose tool the binding gave up on:
// "not measured, the pipeline judges it" (M6.6.1).
const causeKilled = "its fast linter passed the hook's time limit or answered output golaw does not read"

// fed is what a post event feeds back beside its verdict: the findings for
// the lines, the notices of the tools that could not run, for the agent
// and for the person, and the gaps of the files no tool measured.
type fed struct {
	findings []model.Finding
	notices  []string
	person   []string
	gaps     []Gap
}

// certainCheck answers whether a check is one of the certain checks of A27,
// which stay on while the guard is installed even with the feedback off.
// The hook core imports no check package, so the list lives here; the
// proof of the certain denials on the three agents holds it to the checks.
func certainCheck(id string) bool {
	switch id {
	case "surface.code-eval", "surface.hardcoded-credential", "surface.file-length", "surface.inline-script-size",
		"surface.suppression", "surface.analyzer-widening", "docker.tag-and-digest":
		return true
	default:
		return false
	}
}

// feedback gathers what a post event feeds back: the surface notes of its
// decision, only the certain ones when ladder.hook_feedback is off, and,
// while it is on, what the fast tools answer for the files the event
// wrote. Nothing here ever denies (M6.5.3, A28).
func (run execution) feedback(ctx context.Context, in model.Input, decision engine.Decision, dependencies Dependencies) fed {
	var f fed
	if run.request.Event.Shape != model.ShapePost || run.policy == nil {
		return f
	}
	on := run.policy.Ladder.HookFeedback
	for i := range decision.Findings {
		finding := &decision.Findings[i]
		if finding.Severity == model.SeverityNote && surfaceCheck(finding.Check) && (on || certainCheck(finding.Check)) {
			f.findings = append(f.findings, *finding)
		}
	}
	var files []ToolFile
	if on && dependencies.Tools != nil {
		files = run.tools(ctx, in, dependencies, &f)
	}
	if dependencies.State.Dir == "" || !delivered(decision) || len(f.findings) == 0 && len(f.notices) == 0 {
		return f
	}
	return run.once(ctx, f, in, files, dependencies)
}

// tools adds what the fast tools answer for the files the event wrote and
// answers those files, whose bytes the session memory hashes.
func (run execution) tools(ctx context.Context, in model.Input, dependencies Dependencies, f *fed) []ToolFile {
	files, gaps := toolFiles(in)
	if len(files) == 0 {
		return nil
	}
	report := dependencies.Tools(ctx, run.repository, run.policy, files)
	f.findings = append(f.findings, report.Findings...)
	f.notices, f.person = report.Missing, report.Person
	f.gaps = slices.Concat(gaps, pathGaps(report.Killed, causeKilled))
	return files
}

// apply writes the feedback into the result of the event: the lines, and
// on an allow the tool notices beside the notice of the gaps and a line
// for the person; on feedback the notices follow the reason. A result
// that could not judge keeps its own answer.
func (f fed) apply(result Result, language string) Result {
	if result.CannotJudge || len(f.findings) == 0 && len(f.notices) == 0 && len(f.person) == 0 {
		return result
	}
	result.Lines = feedbackLines(f.findings)
	switch result.Outcome {
	case Allow:
		if len(result.Lines) > 0 && result.Human == "" {
			result.Human = feedbackHuman(language, len(result.Lines))
		}
		result.Notice = joinSentences(result.Notice, f.notices)
		result.Human = joinSentences(result.Human, f.person)
	case Feedback:
		result.Reason = joinSentences(result.Reason, f.notices)
	case Deny:
	}
	return result
}

// joinSentences adds sentences after a text, a space between each.
func joinSentences(text string, more []string) string {
	return strings.Join(slices.DeleteFunc(append([]string{text}, more...), func(s string) bool { return s == "" }), " ")
}

// feedbackLines renders the findings as path:line rule (Sonar key) message,
// in report order, twenty per file and the rest counted, so the agent
// reaches every file of the event.
func feedbackLines(findings []model.Finding) []string {
	sorted := slices.Clone(findings)
	model.SortFindings(sorted)
	var lines []string
	for start := 0; start < len(sorted); {
		end := start
		for end < len(sorted) && sorted[end].Path == sorted[start].Path {
			end++
		}
		for i := start; i < min(end, start+maxFileLines); i++ {
			lines = append(lines, feedbackLine(&sorted[i]))
		}
		if rest := end - start - maxFileLines; rest > 0 {
			lines = append(lines, fmt.Sprintf("%s: %d more findings; golaw lint --surface names them all", sorted[start].Path, rest))
		}
		start = end
	}
	return lines
}

// feedbackLine names a linter finding by its rule, the tool's own name the
// agent can look up, and a native one by its check.
func feedbackLine(f *model.Finding) string {
	where := f.Path
	if f.Line > 0 {
		where = fmt.Sprintf("%s:%d", f.Path, f.Line)
	}
	name := f.Check
	if strings.HasPrefix(f.Check, "lint.") {
		name = f.Rule
	}
	sonar := ""
	if f.Sonar != "" {
		sonar = " (" + f.Sonar + ")"
	}
	return where + " " + name + sonar + " " + f.Message
}

// feedbackHuman tells the person how many lines the agent received, in the
// configured language: pt-BR, or English for any other.
func feedbackHuman(language string, n int) string {
	switch {
	case language == model.DefaultLanguage && n == 1:
		return "golaw enviou ao agente 1 apontamento sobre o que ele escreveu; o pipeline julga o mesmo."
	case language == model.DefaultLanguage:
		return fmt.Sprintf("golaw enviou ao agente %d apontamentos sobre o que ele escreveu; o pipeline julga o mesmo.", n)
	case n == 1:
		return "golaw sent the agent 1 finding about what it wrote; the pipeline judges the same."
	default:
		return fmt.Sprintf("golaw sent the agent %d findings about what it wrote; the pipeline judges the same.", n)
	}
}

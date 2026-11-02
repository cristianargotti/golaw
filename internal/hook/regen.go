package hook

import (
	"context"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// RegenReport is what the regeneration of a post event answered (M7.9.4):
// the agent lines (summary, hole open, hole closed, decision, formatted),
// then in Person their person forms in the configured language, in the
// same order, followed by the person forms of the notices, and the notices
// of what could not run ("not regenerated in the hook: run golaw gen" with
// its cause). It never denies (decision 10): the verdict of the event is
// the guard's alone.
type RegenReport struct {
	Lines   []string
	Person  []string
	Notices []string
}

// empty answers whether the report carries nothing for the agent: no line
// and no notice, so the feedback stays as the tools left it.
func (r RegenReport) empty() bool {
	return len(r.Lines) == 0 && len(r.Notices) == 0
}

// regenerate asks the binding to regenerate what a post event wrote, only
// while ladder.hook_regenerate is on (M7.6.2), so a developer who switched
// it off pays nothing and reads nothing. The paths keep the target order,
// which is the order the agent wrote them in.
func (run execution) regenerate(ctx context.Context, in model.Input, dependencies Dependencies) RegenReport {
	if run.request.Event.Shape != model.ShapePost || run.policy == nil || !run.policy.Ladder.HookRegenerate ||
		dependencies.Regenerate == nil || len(in.Targets) == 0 {
		return RegenReport{}
	}
	written := make([]string, 0, len(in.Targets))
	for _, target := range in.Targets {
		written = append(written, target.Path)
	}
	return dependencies.Regenerate(ctx, run.repository, run.policy, written)
}

// onceRegen drops the lines the session already received (M6.6.3), keyed
// by their text, so the same regeneration is told once; the notices stay,
// because a deferred regeneration is news at every event that defers it.
func (run execution) onceRegen(ctx context.Context, report RegenReport, dependencies Dependencies) RegenReport {
	if len(report.Lines) == 0 {
		return report
	}
	if !run.said(ctx, "regen:"+Hash([]byte(strings.Join(report.Lines, "\n"))), dependencies) {
		return report
	}
	report.Person = report.Person[min(len(report.Lines), len(report.Person)):]
	report.Lines = nil
	return report
}

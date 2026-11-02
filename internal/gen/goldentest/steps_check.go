package goldentest

import (
	"context"

	"github.com/cristianargotti/golaw/internal/gen"
)

// check runs golaw gen --check over the module as it stands.
func (h *harness) check(ctx context.Context) (gen.Result, bool) {
	in, err := h.input(ctx)
	if err != nil {
		h.failf(string(StepCheck), "%v", err)
		return gen.Result{}, false
	}
	result, err := gen.Check(ctx, in)
	if err != nil {
		h.failf(string(StepCheck), "%v", err)
		return gen.Result{}, false
	}
	return result, true
}

// checkOpen proves the commands left holes for the agent and that a second
// rendering finds no drift, the in process proof of a deterministic
// template.
func (h *harness) checkOpen(ctx context.Context) {
	result, ok := h.check(ctx)
	if !ok {
		return
	}
	if len(result.Open) == 0 {
		h.failf(string(StepCheck), "no open hole before the fills")
	}
	for _, d := range result.Drift {
		h.failf(string(StepCheck), "drift %s %s", d.Path, d.Cause)
	}
}

// checkClean proves the fills closed every hole and left nothing a golaw
// gen would rewrite.
func (h *harness) checkClean(ctx context.Context) {
	result, ok := h.check(ctx)
	if !ok {
		return
	}
	for _, hole := range result.Open {
		h.failf(string(StepCheck), "hole open %s:%d %s", hole.Path, hole.Line, hole.ID)
	}
	for _, d := range result.Drift {
		h.failf(string(StepCheck), "drift %s %s: %s", d.Path, d.Cause, d.Fix)
	}
}

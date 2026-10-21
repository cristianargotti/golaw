package hook

import (
	"context"

	"github.com/cristianargotti/golaw/internal/model"
)

// answer is the verdict on a decoded request of an enrolled repository:
// its capture recorded first, then the denial its decoder named, the
// baseline an observed event keeps, the names of a write the decoder could
// not hold, or the judgment of the rules.
func (run execution) answer(ctx context.Context, event model.Event, data []byte, dependencies Dependencies) completed {
	run.record(ctx, dependencies)
	answer := completed{event: event, language: run.language()}
	switch {
	case run.request.Denial != "":
		answer.result = rejectionLanguage(event, run.request.Denial, false, run.language())
	case run.request.Observe:
		answer.event, answer.result = run.request.Event, run.observed(ctx, data, dependencies)
	case len(run.request.Gaps) > 0:
		answer.event, answer.result = run.request.Event, run.unmeasured(ctx, dependencies)
	default:
		answer.event, answer.result = run.request.Event, run.judged(ctx, data, dependencies)
	}
	return answer
}

// observed answers a pre event whose policy another event of the host
// judges: the baseline of A15 is kept for its post event and the event is
// allowed, judged by no rule here.
func (run execution) observed(ctx context.Context, data []byte, dependencies Dependencies) Result {
	run.saveBefore(ctx, data, dependencies)
	return Result{Outcome: Allow}
}

// record hands the capture of a request to the dependencies before the
// answer leaves. The record is evidence for doctor, never a verdict, so a
// failed record changes no answer.
func (run execution) record(ctx context.Context, dependencies Dependencies) {
	if run.request.Capture == nil || dependencies.Capture == nil {
		return
	}
	_ = dependencies.Capture(ctx, run.repository, *run.request.Capture) //nolint:errcheck // evidence for doctor, never a verdict
}

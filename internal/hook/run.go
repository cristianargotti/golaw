package hook

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	internalBudget = 8 * time.Second
	hostBudget     = 10 * time.Second
)

// completed is the answer of an evaluation with the language of the
// policy it reached, the default one before a policy is loaded.
type completed struct {
	event    model.Event
	result   Result
	language string
}

// Run bounds the whole invocation with one deadline below the host timeout:
// every callback and the engine honor the context, and an answer that
// arrives after the deadline denies a pre event; a post event keeps known
// findings with a notice, because its tool already ran. The kit starts no
// goroutine (its own goleak rule), so a reader that never returns is bounded
// by the host timeout and the fail-closed mode the installer configures.
func Run(ctx context.Context, agent string, input io.Reader, adapter Adapter, dependencies Dependencies) Reply {
	if adapter.Parse == nil || adapter.Encode == nil {
		return Reply{ExitCode: 2, Stderr: "cannot judge: incomplete adapter\n"}
	}
	budget := dependencies.Budget
	if budget == 0 {
		budget = internalBudget
	}
	if budget < 0 || budget >= hostBudget {
		return adapter.Encode(model.Event{}, rejection(model.Event{},
			"cannot judge: internal deadline must be below ten seconds", true))
	}
	bounded, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	answer := evaluateSafely(bounded, agent, input, adapter, dependencies)
	if bounded.Err() != nil {
		return lateReply(adapter, answer)
	}
	return adapter.Encode(answer.event, answer.result)
}

// lateReply keeps established findings after execution with a notice of
// what the deadline left unmeasured; without a finding it leaves the work
// to the pipeline. Every pre event stays denied because it has not run yet.
func lateReply(adapter Adapter, answer completed) Reply {
	if answer.event.Shape == model.ShapePost {
		language := answer.language
		if language == "" {
			language = model.DefaultLanguage
		}
		gaps := []Gap{{Cause: causeDeadline}}
		if answer.result.enforcedFinding {
			answer.result.CannotJudge = false
			return adapter.Encode(answer.event, withGaps(answer.result, gaps, language))
		}
		return adapter.Encode(answer.event, noticed(gaps, language))
	}
	return deadlineReply(adapter)
}

// evaluateSafely reads the object and evaluates it; a panic becomes a
// cannot-judge result instead of a crash the host would fail open on.
func evaluateSafely(ctx context.Context, agent string, input io.Reader, adapter Adapter, dependencies Dependencies) completed {
	var answer completed
	func() {
		defer func() {
			if recover() != nil {
				answer = completed{result: rejection(model.Event{},
					"cannot judge: hook evaluation panicked", true)}
			}
		}()
		answer = evaluate(ctx, agent, input, adapter, dependencies)
	}()
	return answer
}

func evaluate(ctx context.Context, agent string, input io.Reader, adapter Adapter, dependencies Dependencies) completed {
	data, err := ReadObject(input)
	if errors.Is(err, model.ErrTooLarge) {
		return evaluateOversized(ctx, agent, io.MultiReader(bytes.NewReader(data), input), adapter, dependencies)
	}
	if err != nil {
		return completed{result: failure(model.Event{}, err)}
	}
	return evaluateEvent(ctx, agent, data, adapter, dependencies)
}

func deadlineReply(adapter Adapter) Reply {
	err := fmt.Errorf("%w: hook evaluation exceeded its budget", model.ErrDeadline)
	return adapter.Encode(model.Event{}, failure(model.Event{}, err))
}

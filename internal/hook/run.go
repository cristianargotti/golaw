package hook

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	internalBudget = 8 * time.Second
	hostBudget     = 10 * time.Second
)

type completed struct {
	event  model.Event
	result Result
}

// Run bounds the whole invocation with one deadline below the host timeout:
// every callback and the engine honor the context, and an answer that
// arrives after the deadline is replaced by a denial, so a late answer never
// reaches the agent as an allow. The kit starts no goroutine (its own
// goleak rule), so a reader that never returns is bounded by the host
// timeout and the fail-closed mode the installer configures.
func Run(ctx context.Context, agent string, input io.Reader, adapter Adapter, dependencies Dependencies) Reply {
	if adapter.Header == nil || adapter.Decode == nil || adapter.Encode == nil {
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
		return deadlineReply(adapter)
	}
	return adapter.Encode(answer.event, answer.result)
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
	if err != nil {
		return completed{result: failure(model.Event{}, err)}
	}
	event, result := Evaluate(ctx, agent, data, adapter, dependencies)
	return completed{event: event, result: result}
}

func deadlineReply(adapter Adapter) Reply {
	err := fmt.Errorf("%w: hook evaluation exceeded its budget", model.ErrDeadline)
	return adapter.Encode(model.Event{}, failure(model.Event{}, err))
}

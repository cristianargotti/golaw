package hook

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	captureBudget  = 7 * time.Second
	captureReserve = time.Second
)

// captureIdentities hashes what the changes of the repository name and
// the paths of also beside them, or every file of the snapshot without
// the changes, inside the capture budget; the files are read in memory
// and the context is checked between files, so the reserve for the
// answer is never consumed. It answers the changes it read them from.
func (run execution) captureIdentities(ctx context.Context, dependencies Dependencies, also []string) (map[string]string, Changes, error) {
	bounded, cancel := captureContext(ctx)
	defer cancel()
	var files map[string]string
	changes := Changes{Snapshot: run.base}
	var err error
	func() {
		defer func() {
			if recover() != nil {
				err = fmt.Errorf("%w: content capture panicked", model.ErrCannotJudge)
			}
		}()
		var paths []string
		if dependencies.Changes != nil {
			changes, err = dependencies.Changes(bounded, run.repository, also)
			// Never nil, so a deleted path is identified as absent instead
			// of the whole snapshot being listed.
			paths = make([]string, 0, len(changes.Paths)+len(also))
			paths = append(append(paths, changes.Paths...), also...)
			slices.Sort(paths)
			paths = slices.Compact(paths)
		}
		if err == nil {
			files, err = Identities(bounded, changes.Snapshot, paths)
		}
	}()
	if err != nil {
		return nil, Changes{}, fmt.Errorf("optional content capture: %w", err)
	}
	return files, changes, nil
}

func captureContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if deadline, ok := ctx.Deadline(); ok {
		return context.WithDeadline(ctx, deadline.Add(-captureReserve))
	}
	return context.WithTimeout(ctx, captureBudget)
}

// optionalAction runs an action that may fail or panic without changing
// the verdict: it gets the bounded context and its outcome is dropped.
func optionalAction(ctx context.Context, action func(context.Context)) {
	bounded, cancel := captureContext(ctx)
	defer cancel()
	defer func() {
		// An optional action never changes the verdict, so its panic is dropped.
		if recover() != nil {
			return
		}
	}()
	action(bounded)
}

// sweepState sweeps the state within the answer reserve; a sweep that
// cannot finish in time is reported as a deadline and never blocks the
// verdict.
func sweepState(ctx context.Context, store Store) error {
	bounded, cancel := context.WithTimeout(ctx, captureReserve)
	defer cancel()
	var err error
	func() {
		defer func() {
			if recover() != nil {
				err = fmt.Errorf("%w: state sweep panicked", model.ErrCannotJudge)
			}
		}()
		err = store.Sweep(bounded)
	}()
	return err
}

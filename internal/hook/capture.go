package hook

import (
	"context"
	"fmt"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	captureBudget  = 7 * time.Second
	captureReserve = time.Second
)

// captureIdentities hashes every file of the snapshot inside the capture
// budget; the snapshot is read in memory and checks the context between
// files, so the reserve for the answer is never consumed.
func captureIdentities(ctx context.Context, snapshot model.Snapshot) (map[string]string, error) {
	bounded, cancel := captureContext(ctx)
	defer cancel()
	var files map[string]string
	var err error
	func() {
		defer func() {
			if recover() != nil {
				err = fmt.Errorf("%w: content capture panicked", model.ErrCannotJudge)
			}
		}()
		files, err = Identities(bounded, snapshot, nil)
	}()
	if err != nil {
		return nil, fmt.Errorf("optional content capture: %w", err)
	}
	return files, nil
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

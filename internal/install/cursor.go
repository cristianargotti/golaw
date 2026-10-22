package install

import "fmt"

// PlanCursor plans the Cursor installation of section 8.5, a compatibility
// preview: the handlers merged into the hooks file of the scope, the
// project rule with --scope project, the shared skill and the receipt,
// with the note that the writes are captured and not yet enforced.
// Planning reads and never writes; Apply shows the diff and enrolls.
func PlanCursor(options Options) (Plan, error) {
	plan, previous, err := cursorPlan(options)
	if err != nil {
		return Plan{}, err
	}
	options.Scope = plan.Receipt.Scope
	for _, step := range []func() error{
		func() error { return cursorHooks(&plan, options, previous) },
		func() error { return cursorFiles(&plan, options, previous) },
		func() error { return cursorReceipt(&plan) },
	} {
		if err := step(); err != nil {
			return Plan{}, err
		}
	}
	plan.Notes = append(plan.Notes, cursorPreviewNote)
	return plan, nil
}

// UninstallCursor plans the removal of what a Cursor receipt lists.
func UninstallCursor(receiptPath string) (Plan, error) {
	return Plan{}, fmt.Errorf("cursor receipt %s: %w", receiptPath, ErrNotBuilt)
}

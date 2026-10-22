package install

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

// UninstallCursor plans the removal of exactly what a Cursor receipt
// lists: its handlers, its project rule, the shared skill when no other
// agent still lists it, and the receipt last. Without a receipt nothing
// is guessed, and the enrollment of the repository stays.
func UninstallCursor(receiptPath string) (Plan, error) {
	receipt, err := ReadReceipt(receiptPath)
	if err != nil {
		return Plan{}, err
	}
	stateDir, err := cursorStateDir(receiptPath, receipt)
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{Receipt: receipt, ReceiptPath: receiptPath, Uninstall: true}
	for _, step := range []func() error{
		func() error { return cursorRestoreHooks(&plan) },
		func() error { return cursorRemoveFiles(&plan, stateDir) },
		func() error { return addChange(&plan, receiptPath, nil, true) },
	} {
		if err := step(); err != nil {
			return Plan{}, err
		}
	}
	return plan, nil
}

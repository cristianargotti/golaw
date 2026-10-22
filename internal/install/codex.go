package install

// codexProjectNote is the step a project installation needs on top of the
// hook review: Codex loads the .codex folder of a project only once the
// person trusts that project.
const codexProjectNote = "Confie no projeto no Codex para carregar a configuração local."

// PlanCodex plans the Codex installation of section 8.5: the handlers
// merged into the hooks file, the rules file with --scope project, the
// shared skill, the marked block of the instruction file and the receipt,
// with the trust steps the person takes after the write. Planning reads
// and never writes, and the trust record of Codex is only ever read.
func PlanCodex(options Options) (Plan, error) {
	plan, previous, err := prepareCodex(options)
	if err != nil {
		return Plan{}, err
	}
	options.Scope = plan.Receipt.Scope
	for _, step := range []func() error{
		func() error { return addCodexHooks(&plan, previous, options) },
		func() error { return addCodexArtifacts(&plan, previous, options) },
		func() error { return codexTrustNotes(&plan, options) },
		func() error { return finishCodexReceipt(&plan) },
	} {
		if err := step(); err != nil {
			return Plan{}, err
		}
	}
	if plan.Receipt.Scope == scopeProject {
		plan.Notes = append(plan.Notes, codexProjectNote)
	}
	return plan, nil
}

// UninstallCodex plans the removal of exactly what a Codex receipt lists:
// its handlers, its rules file, the shared skill when no other agent still
// lists it, its marked block, and the receipt last. Without a receipt
// nothing is guessed, and the trust record of Codex is never touched.
func UninstallCodex(receiptPath string) (Plan, error) {
	receipt, err := ReadReceipt(receiptPath)
	if err != nil {
		return Plan{}, err
	}
	stateDir, err := codexReceiptState(receiptPath, receipt)
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{Receipt: receipt, ReceiptPath: receiptPath, Uninstall: true}
	if err := removeCodexHooks(&plan); err != nil {
		return Plan{}, err
	}
	if err := removeCodexArtifacts(&plan, stateDir); err != nil {
		return Plan{}, err
	}
	if err := addChange(&plan, receiptPath, nil, true); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

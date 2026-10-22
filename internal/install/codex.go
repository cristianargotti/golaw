package install

import "fmt"

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

// UninstallCodex plans the removal of what a Codex receipt lists.
func UninstallCodex(receiptPath string) (Plan, error) {
	return Plan{}, fmt.Errorf("codex receipt %s: %w", receiptPath, ErrNotBuilt)
}

package install

import "fmt"

// PlanCodex plans the Codex installation of section 8.5.
func PlanCodex(options Options) (Plan, error) {
	return Plan{}, fmt.Errorf("codex in %s: %w", options.Root, ErrNotBuilt)
}

// UninstallCodex plans the removal of what a Codex receipt lists.
func UninstallCodex(receiptPath string) (Plan, error) {
	return Plan{}, fmt.Errorf("codex receipt %s: %w", receiptPath, ErrNotBuilt)
}

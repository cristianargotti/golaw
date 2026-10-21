package install

import "fmt"

// PlanCursor plans the Cursor installation of section 8.5, a compatibility
// preview.
func PlanCursor(options Options) (Plan, error) {
	return Plan{}, fmt.Errorf("cursor in %s: %w", options.Root, ErrNotBuilt)
}

// UninstallCursor plans the removal of what a Cursor receipt lists.
func UninstallCursor(receiptPath string) (Plan, error) {
	return Plan{}, fmt.Errorf("cursor receipt %s: %w", receiptPath, ErrNotBuilt)
}

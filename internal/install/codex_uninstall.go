package install

import (
	"fmt"
	"path/filepath"
)

// codexReceiptState checks that a receipt is a Codex receipt kept where a
// Codex installation keeps it, <state>/codex/user/receipt.json or
// <state>/codex/project/<identity>/receipt.json with its bundle beside it,
// and answers the state directory: the other receipts that may still own
// the shared skill are read from there, so a receipt copied elsewhere must
// never decide which directory that is.
func codexReceiptState(receiptPath string, receipt Receipt) (string, error) {
	store := filepath.Dir(receiptPath)
	valid := receipt.Agent == codexAgent && filepath.IsAbs(receiptPath) &&
		filepath.Base(receiptPath) == receiptName && receipt.Bundle == store &&
		filepath.IsAbs(receipt.HooksPath) && filepath.Base(receipt.HooksPath) == codexHooksName
	switch receipt.Scope {
	case scopeProject:
		valid = valid && filepath.Base(store) == receipt.ProjectID
		store = filepath.Dir(store)
	case scopeUser:
	default:
		valid = false
	}
	agentDir := filepath.Dir(store)
	if !valid || filepath.Base(store) != receipt.Scope || filepath.Base(agentDir) != codexAgent {
		return "", fmt.Errorf("receipt %s is not where a codex installation keeps one: %w", receiptPath, ErrSettings)
	}
	return filepath.Dir(agentDir), nil
}

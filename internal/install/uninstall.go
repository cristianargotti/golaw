package install

import (
	"fmt"
	"path/filepath"
)

// PlanUninstall refuses to infer ownership when a receipt is absent.
func PlanUninstall(receiptPath string) (Plan, error) {
	receipt, err := ReadReceipt(receiptPath)
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{Receipt: receipt, ReceiptPath: receiptPath, Uninstall: true}
	for _, artifact := range receipt.Artifacts {
		if artifact.Path == receipt.SettingsPath {
			continue
		}
		if err := removeArtifact(&plan, artifact); err != nil {
			return Plan{}, err
		}
	}
	if err := restoreSettingsChange(&plan); err != nil {
		return Plan{}, err
	}
	if err := addChange(&plan, receiptPath, nil, true); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

func removeArtifact(plan *Plan, artifact Artifact) error {
	relative, err := filepath.Rel(plan.Receipt.Bundle, artifact.Path)
	if err != nil || !filepath.IsLocal(relative) || relative == "." {
		return fmt.Errorf("receipt artifact destination: %w", ErrPath)
	}
	data, exists, err := readOptional(artifact.Path)
	if err != nil {
		return err
	}
	if exists && Digest(data) != artifact.SHA256 {
		return changedArtifact(artifact.Path, ErrChanged)
	}
	return addChange(plan, artifact.Path, nil, true)
}

func restoreSettingsChange(plan *Plan) error {
	data, exists, err := readOptional(plan.Receipt.SettingsPath)
	if err != nil || !exists {
		return err
	}
	restored, err := RestoreSettings(data, plan.Receipt.Ownership)
	if err != nil {
		return err
	}
	return addChange(plan, plan.Receipt.SettingsPath, restored, restored == nil)
}

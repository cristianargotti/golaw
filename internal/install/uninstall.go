package install

import (
	"fmt"
	"path/filepath"
	"slices"
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
	if err := releaseSettings(&plan); err != nil {
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

// releaseSettings restores the settings file the receipt names and takes
// the exclude line out once that file is gone.
func releaseSettings(plan *Plan) error {
	removed, err := settingsRemoved(plan)
	if err != nil {
		return err
	}
	return removeExclude(plan, removed)
}

// settingsRemoved restores the settings file the receipt names and
// reports whether it is gone after the uninstall: absent already, or
// removed because the installation created it and nothing of the person
// is left in it.
func settingsRemoved(plan *Plan) (bool, error) {
	path := plan.Receipt.SettingsPath
	_, exists, err := readOptional(path)
	if err != nil {
		return false, err
	}
	if err := restoreSettingsChange(plan, path, plan.Receipt.Ownership); err != nil {
		return false, err
	}
	return !exists || slices.ContainsFunc(plan.Changes, func(change Change) bool {
		return change.Path == path && change.Remove
	}), nil
}

// restoreSettingsChange plans the settings file at path without the
// entries owned records, removing it when the installation created it
// and nothing else is left; an absent file needs nothing.
func restoreSettingsChange(plan *Plan, path string, owned Ownership) error {
	data, exists, err := readOptional(path)
	if err != nil || !exists {
		return err
	}
	restored, err := RestoreSettings(data, owned)
	if err != nil {
		return err
	}
	return addChange(plan, path, restored, restored == nil)
}

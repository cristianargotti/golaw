package install

import (
	"fmt"
	"os"
	"path/filepath"
)

// cursorStateDir checks that a receipt is a Cursor receipt kept where a
// Cursor installation keeps it, <state>/cursor/user/receipt.json or
// <state>/cursor/project/<identity>/receipt.json with its bundle beside
// it and its hooks in a .cursor folder, and answers the state directory:
// the other receipts that may still own the shared skill are read from
// there, so a receipt copied elsewhere must never decide which one it is.
func cursorStateDir(receiptPath string, receipt Receipt) (string, error) {
	levels := 3
	parts := []string{cursorAgent, receipt.Scope, receiptName}
	if receipt.Scope == scopeProject {
		levels, parts = 4, []string{cursorAgent, scopeProject, receipt.ProjectID, receiptName}
	}
	stateDir := receiptPath
	for range levels {
		stateDir = filepath.Dir(stateDir)
	}
	valid := receipt.Agent == cursorAgent && filepath.IsAbs(receiptPath) &&
		(receipt.Scope == scopeUser || receipt.Scope == scopeProject) &&
		filepath.Join(append([]string{stateDir}, parts...)...) == receiptPath &&
		receipt.Bundle == filepath.Dir(receiptPath) && filepath.IsAbs(receipt.HooksPath) &&
		filepath.Base(receipt.HooksPath) == cursorHooksName &&
		filepath.Base(filepath.Dir(receipt.HooksPath)) == cursorLocalDir
	if !valid {
		return "", fmt.Errorf("receipt %s is not a receipt a cursor installation keeps there: %w", receiptPath, ErrPath)
	}
	return stateDir, nil
}

// cursorRemoveFiles plans the removal of the files a Cursor receipt owns
// whole. Each is removed only while its bytes are the ones the receipt
// recorded, so an edit the person made is refused and kept; one already
// gone needs nothing. A path no Cursor installation writes is refused,
// because removing it would act on evidence golaw did not produce.
func cursorRemoveFiles(plan *Plan, stateDir string) error {
	for _, artifact := range plan.Receipt.Artifacts {
		var err error
		switch {
		case plan.Receipt.Scope == scopeProject && artifact.Path == cursorRulePath(plan.Receipt):
			err = cursorRemoveRule(plan, artifact)
		case cursorSkillOf(plan.Receipt, artifact.Path):
			err = cursorUnlinked(artifact.Path, cursorSkillDepth)
			if err == nil {
				err = removeSharedSkill(plan, stateDir, cursorAgent, artifact)
			}
		default:
			err = fmt.Errorf("receipt artifact %s is no file a cursor installation writes: %w", artifact.Path, ErrPath)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func cursorRemoveRule(plan *Plan, artifact Artifact) error {
	if err := cursorUnlinked(artifact.Path, cursorRuleDepth); err != nil {
		return err
	}
	current, exists, err := readOptional(artifact.Path)
	if err != nil || !exists {
		return err
	}
	if Digest(current) != artifact.SHA256 {
		return fmt.Errorf("the cursor rule %s changed since the install: restore it or remove it: %w", artifact.Path, ErrChanged)
	}
	return addChange(plan, artifact.Path, nil, true)
}

// cursorSkillOf recognizes the shared skill by the folders both hosts read
// it from and, with --scope project, by the repository whose .cursor folder
// holds the hooks, so a receipt never names another project's skill or a
// legacy location as golaw's.
func cursorSkillOf(receipt Receipt, path string) bool {
	if !codexSkillPath(path) {
		return false
	}
	if receipt.Scope != scopeProject {
		return true
	}
	base := path
	for range cursorSkillDepth {
		base = filepath.Dir(base)
	}
	root := filepath.Dir(filepath.Dir(receipt.HooksPath))
	if base == root {
		return true
	}
	baseInfo, baseErr := os.Stat(base)
	rootInfo, rootErr := os.Stat(root)
	return baseErr == nil && rootErr == nil && os.SameFile(baseInfo, rootInfo)
}

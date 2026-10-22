package install

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

// listingReceipt is a receipt of the state that lists the shared skill,
// with the agent directory it was read from and its path.
type listingReceipt struct {
	agent, path string
	receipt     Receipt
}

// SkillOwners answers the agents other than agent whose receipt of the
// scope lists path among its artifacts, sorted. Uninstall removes the
// shared skill only when this list is empty, so one agent never takes away
// the instructions another agent still loads every session.
func SkillOwners(stateDir, scope, identity, agent, path string) ([]string, error) {
	listing, err := listingReceipts(stateDir, scope, identity, path)
	if err != nil {
		return nil, err
	}
	var owners []string
	for index := range listing {
		owner := listing[index].agent
		if owner != agent && !slices.Contains(owners, owner) {
			owners = append(owners, owner)
		}
	}
	slices.Sort(owners)
	return owners, nil
}

// skillReceipts answers the receipts of one scope that list path, keyed by
// the receipt path, the receipt of every agent included.
func skillReceipts(stateDir, scope, identity, path string) (map[string]Receipt, error) {
	listing, err := listingReceipts(stateDir, scope, identity, path)
	if err != nil {
		return nil, err
	}
	receipts := make(map[string]Receipt, len(listing))
	for index := range listing {
		receipts[listing[index].path] = listing[index].receipt
	}
	return receipts, nil
}

// listingReceipts reads the receipt each immediate agent directory of the
// state keeps for the scope and answers those that list path. At user
// scope the receipt of each agent counts whatever repository it was
// written from, because every repository shares the one user skill; at
// project scope only the receipts of that project identity do. No other
// state file is read, so nothing else ever becomes ownership evidence.
func listingReceipts(stateDir, scope, identity, path string) ([]listingReceipt, error) {
	if stateDir == "" {
		return nil, nil
	}
	if scope == scopeProject && identity == "" {
		return nil, fmt.Errorf("project identity of the shared skill: %w", ErrPath)
	}
	entries, err := os.ReadDir(stateDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read installation state: %w", err)
	}
	var listing []listingReceipt
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		receiptPath, receipt, found, err := scopeReceipt(stateDir, entry.Name(), scope, identity)
		if err != nil {
			return nil, err
		}
		if found && slices.ContainsFunc(receipt.Artifacts, func(artifact Artifact) bool {
			return artifact.Path == path
		}) {
			listing = append(listing, listingReceipt{agent: entry.Name(), path: receiptPath, receipt: receipt})
		}
	}
	return listing, nil
}

// scopeReceipt reads the receipt an agent directory keeps for the scope.
// No link on the way is followed: the installer writes plain directories
// there, so a link would make a file of another tree the evidence that
// keeps a skill installed or lets it go.
func scopeReceipt(stateDir, agent, scope, identity string) (string, Receipt, bool, error) {
	parts := []string{agent, scopeUser, receiptName}
	if scope == scopeProject {
		parts = []string{agent, scopeProject, identity, receiptName}
	}
	path := stateDir
	for _, part := range parts {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			return path, Receipt{}, false, nil
		}
		if err != nil {
			return path, Receipt{}, false, fmt.Errorf("inspect receipt: %w", err)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return path, Receipt{}, false, changedArtifact(path, ErrPath)
		}
	}
	receipt, err := ReadReceipt(path)
	if err != nil {
		return path, Receipt{}, false, changedArtifact(path, err)
	}
	return path, receipt, true, nil
}

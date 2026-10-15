package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
)

// Options contains resolved command inputs rather than environment reads.
type Options struct {
	Root, ConfigDir, StateDir, Scope, Version string
	PolicyDigest, ProfileDigest               string
	ReadRules                                 []string
}

// Change freezes the comparison required before a write.
type Change struct {
	Path          string
	Before, After []byte
	Existed       bool
	Remove        bool
}

// Plan keeps the complete diff available before enrollment or file writes.
type Plan struct {
	Changes                     []Change
	Receipt                     Receipt
	ReceiptPath                 string
	PolicyDigest, ProfileDigest string
	Uninstall                   bool
}

// PlanClaude builds an offline installation from the validated embedded bundle.
func PlanClaude(options Options) (Plan, error) {
	plan, previous, err := preparePlan(options)
	if err != nil {
		return Plan{}, err
	}
	err = addBundle(&plan, previous)
	if err != nil {
		return Plan{}, err
	}
	err = addSettings(&plan, options.ReadRules, previous.Ownership)
	if err != nil {
		return Plan{}, err
	}
	encoded, err := json.MarshalIndent(plan.Receipt, "", "  ")
	if err != nil {
		return Plan{}, fmt.Errorf("encode receipt: %w", err)
	}
	if err := addChange(&plan, plan.ReceiptPath, append(encoded, '\n'), false); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

func preparePlan(options Options) (Plan, Receipt, error) {
	root, identity, err := ProjectIdentity(options.Root)
	if err != nil {
		return Plan{}, Receipt{}, err
	}
	scope, store, config, err := installRoots(options, root, identity)
	if err != nil {
		return Plan{}, Receipt{}, err
	}
	path := filepath.Join(store, "receipt.json")
	old, err := ReadReceipt(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Plan{}, Receipt{}, err
	}
	receipt := Receipt{
		Schema: 1, ProjectID: identity, Scope: scope, Version: options.Version,
		SettingsPath: filepath.Join(config, "settings.json"),
		Bundle:       filepath.Join(store, "marketplace"),
	}
	plan := Plan{
		Receipt: receipt, ReceiptPath: path,
		PolicyDigest: options.PolicyDigest, ProfileDigest: options.ProfileDigest,
	}
	return plan, old, nil
}

func installRoots(options Options, root, identity string) (string, string, string, error) {
	scope := options.Scope
	if scope == "" {
		scope = "user"
	}
	if scope != "user" && scope != "project" {
		return "", "", "", fmt.Errorf("scope: %w", ErrPath)
	}
	store := filepath.Join(options.StateDir, "claude", scope)
	config := options.ConfigDir
	if scope == "project" {
		store = filepath.Join(store, identity)
		config = filepath.Join(root, ".claude")
	}
	if !filepath.IsAbs(store) || !filepath.IsAbs(config) {
		return "", "", "", fmt.Errorf("installation roots: %w", ErrPath)
	}
	return scope, store, config, nil
}

func addSettings(plan *Plan, rules []string, previous Ownership) error {
	current, _, err := readOptional(plan.Receipt.SettingsPath)
	if err != nil {
		return err
	}
	desired, owned, err := MergeSettings(current, plan.Receipt.Bundle, rules, previous)
	if err != nil {
		return err
	}
	plan.Receipt.Ownership = owned
	return addArtifact(plan, plan.Receipt.SettingsPath, desired)
}

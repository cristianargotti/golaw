package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
)

// The settings files of a Claude installation, by scope.
const (
	userSettingsName    = "settings.json"
	projectSettingsName = "settings.local.json"
)

// The lines a Claude project installation ends with: Claude Code reads the
// marketplace of the project and local settings only in a workspace the
// person trusted, and an install that moved the entries out of the shared
// settings.json leaves a change to commit when git holds that file.
const (
	claudeProjectNote = "Confie no projeto no Claude Code para carregar o plugin do golaw."
	claudeMovedNote   = "As entradas do golaw saíram de %s e foram para %s; faça commit da mudança se o git guarda o primeiro."
)

// Options contains resolved command inputs rather than environment reads.
// Home is the home directory of the person, Executable the path of the
// running binary the hook command names and GOOS the operating system the
// hook command is written for. ConfigDirSet says the person named
// ConfigDir through the override variable of the agent's host.
type Options struct {
	Root, ConfigDir, StateDir, Scope, Version string
	PolicyDigest, ProfileDigest               string
	ReadRules                                 []string
	Home, Executable, GOOS                    string
	ConfigDirSet                              bool
}

// Change freezes the comparison required before a write.
type Change struct {
	Path          string
	Before, After []byte
	Existed       bool
	Remove        bool
}

// Plan keeps the complete diff available before enrollment or file writes.
// Notes are the lines the person reads after the write, such as the trust
// step of a host that keys trust by the hook definition.
type Plan struct {
	Changes                     []Change
	Receipt                     Receipt
	ReceiptPath                 string
	PolicyDigest, ProfileDigest string
	Uninstall                   bool
	Notes                       []string
}

// PlanClaude plans from the embedded bundle alone, so the plugin it installs
// is the one this binary was built and validated with, and nothing is
// downloaded.
func PlanClaude(options Options) (Plan, error) {
	plan, previous, err := preparePlan(options)
	if err != nil {
		return Plan{}, err
	}
	err = addBundle(&plan, previous)
	if err != nil {
		return Plan{}, err
	}
	err = addSettings(&plan, options.ReadRules, previous)
	if err != nil {
		return Plan{}, err
	}
	err = addExclude(&plan, previous)
	if err != nil {
		return Plan{}, err
	}
	if plan.Receipt.Scope == scopeProject {
		plan.Notes = append(plan.Notes, claudeProjectNote)
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
		SettingsPath: filepath.Join(config, claudeSettingsName(scope)),
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

// claudeSettingsName is the settings file a Claude installation writes:
// the user settings.json at user scope, and at project scope the
// settings.local.json Claude Code keeps per person and never shares,
// because the entries name the marketplace under the person's state
// directory, a path no teammate has.
func claudeSettingsName(scope string) string {
	if scope == scopeProject {
		return projectSettingsName
	}
	return userSettingsName
}

// SharedSettings reports whether a Claude receipt names the settings.json
// of the repository, where a project installation before
// settings.local.json wrote its entries; the next install moves them.
func SharedSettings(receipt Receipt) bool {
	return receipt.Agent == "" && receipt.Scope == scopeProject &&
		filepath.Base(receipt.SettingsPath) != projectSettingsName
}

// addSettings merges the owned entries into the settings file of the scope.
// When the previous receipt is a project receipt that names the shared
// settings.json, the entries it owns there are released first, with
// everything the person wrote kept, and settings.local.json starts its own
// ownership record.
func addSettings(plan *Plan, rules []string, previous Receipt) error {
	prior := previous.Ownership
	if SharedSettings(previous) {
		if err := restoreSettingsChange(plan, previous.SettingsPath, previous.Ownership); err != nil {
			return err
		}
		prior = Ownership{}
		plan.Notes = append(plan.Notes, fmt.Sprintf(claudeMovedNote, previous.SettingsPath, plan.Receipt.SettingsPath))
	}
	current, _, err := readOptional(plan.Receipt.SettingsPath)
	if err != nil {
		return err
	}
	desired, owned, err := MergeSettings(current, plan.Receipt.Bundle, rules, prior)
	if err != nil {
		return err
	}
	plan.Receipt.Ownership = owned
	return addArtifact(plan, plan.Receipt.SettingsPath, desired)
}

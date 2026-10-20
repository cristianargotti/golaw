package setup

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/cristianargotti/golaw/internal/install"
)

// PlanUninstall requires ownership evidence before any hook can be removed.
func PlanUninstall(ctx context.Context, options Options, deps Dependencies) (Plan, error) {
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	plan, err := uninstallReceipt(options)
	if err != nil || plan.Receipt.Schema == 0 {
		return plan, err
	}
	plan.Changes, err = inspectUninstall(ctx, plan, deps)
	return plan, err
}

func uninstallReceipt(options Options) (Plan, error) {
	root, identity, err := install.ProjectIdentity(options.Root)
	if err != nil {
		return Plan{}, fmt.Errorf("setup uninstall project identity: %w", err)
	}
	if !filepath.IsAbs(options.StateDir) {
		return Plan{}, fmt.Errorf("setup state directory must be absolute: %w", install.ErrPath)
	}
	options.Root, options.Env = root, slices.Clone(options.Env)
	path := ReceiptPath(options.StateDir, identity)
	receipt, err := ReadReceipt(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Plan{Uninstall: true}, nil
	}
	if err != nil {
		return Plan{}, err
	}
	if receipt.Identity != identity || receipt.Root != root {
		return Plan{}, fmt.Errorf("receipt %q belongs to another project; inspect it before uninstalling: %w", path, install.ErrChanged)
	}
	return Plan{Options: options, Receipt: receipt, ReceiptPath: path, Uninstall: true}, nil
}

func inspectUninstall(ctx context.Context, plan Plan, deps Dependencies) ([]install.Change, error) {
	if err := uninstallOwnership(plan); err != nil {
		return nil, err
	}
	hooks, err := hooksDirectory(ctx, plan.Options, deps)
	if err != nil {
		return nil, fmt.Errorf("inspect uninstall routing: %w", err)
	}
	if hooks != plan.Receipt.HooksPath {
		return nil, fmt.Errorf("hooks routing changed from %q to %q; restore the recorded routing before uninstalling: %w",
			plan.Receipt.HooksPath, hooks, install.ErrChanged)
	}
	return uninstallChanges(plan.Receipt)
}

func uninstallOwnership(plan Plan) error {
	names := []string{"pre-commit", "commit-msg", "pre-push"}
	if len(plan.Receipt.Hooks) != len(names) {
		return fmt.Errorf("receipt %q has incomplete hook ownership; inspect it before uninstalling: %w", plan.ReceiptPath, install.ErrChanged)
	}
	for _, name := range names {
		path := filepath.Join(plan.Receipt.HooksPath, name)
		if !slices.ContainsFunc(plan.Receipt.Hooks, func(hook install.Artifact) bool { return hook.Path == path }) {
			return fmt.Errorf("receipt %q does not own %q; inspect it before uninstalling: %w", plan.ReceiptPath, path, install.ErrChanged)
		}
	}
	return nil
}

func uninstallChanges(receipt Receipt) ([]install.Change, error) {
	info, err := os.Lstat(receipt.HooksPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("inspect hooks directory %q: %w", receipt.HooksPath, err)
	}
	if err == nil && !info.IsDir() {
		return nil, fmt.Errorf("hooks directory %q changed; restore it before uninstalling: %w", receipt.HooksPath, install.ErrChanged)
	}
	changes := make([]install.Change, 0, len(receipt.Hooks))
	for _, hook := range receipt.Hooks {
		data, exists, readErr := readHook(hook.Path)
		if readErr != nil {
			return nil, fmt.Errorf("hook %q changed; inspect it before uninstalling: %w: %w", hook.Path, install.ErrChanged, readErr)
		}
		if !exists {
			continue
		}
		if install.Digest(data) != hook.SHA256 {
			return nil, fmt.Errorf("hook %q changed; copy your edits elsewhere, run golaw setup to re-arm the hook, then run golaw setup --uninstall: %w",
				hook.Path, install.ErrChanged)
		}
		changes = append(changes, install.Change{Path: hook.Path, Before: data, Existed: true, Remove: true})
	}
	return changes, nil
}

func recheckUninstall(ctx context.Context, plan Plan, deps Dependencies) (Plan, error) {
	current, err := PlanUninstall(ctx, plan.Options, deps)
	if err != nil {
		return Plan{}, err
	}
	if current.ReceiptPath != plan.ReceiptPath || !sameSetupReceipt(current.Receipt, plan.Receipt) {
		return Plan{}, fmt.Errorf("receipt %q changed; plan uninstall again: %w", plan.ReceiptPath, install.ErrChanged)
	}
	return current, nil
}

func sameSetupReceipt(left, right Receipt) bool {
	return left.Schema == right.Schema && left.Identity == right.Identity && left.Root == right.Root &&
		left.Version == right.Version && left.HooksPath == right.HooksPath && left.Golaw == right.Golaw &&
		left.ArmedAt.Equal(right.ArmedAt) && slices.Equal(left.Tools, right.Tools) && slices.Equal(left.Hooks, right.Hooks)
}

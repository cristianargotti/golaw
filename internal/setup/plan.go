package setup

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/runner"
)

// Options makes the caller own environment and clock selection.
type Options struct {
	Root, StateDir, Version, BinDir string
	Env                             []string
	Now                             time.Time
}

// Dependencies keeps installation tests independent of external tools.
type Dependencies struct {
	Run    func(context.Context, runner.Command) (runner.Output, error)
	Verify func(context.Context, runner.Tool, []string) (string, error)
}

// Plan freezes hook inputs before any installation can change them.
type Plan struct {
	Options     Options
	Receipt     Receipt
	ReceiptPath string
	Changes     []install.Change
	Uninstall   bool
}

// PlanInstall refuses known broken routing before tools can be installed.
func PlanInstall(ctx context.Context, options Options, deps Dependencies) (Plan, error) {
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	root, identity, err := install.ProjectIdentity(options.Root)
	if err != nil {
		return Plan{}, fmt.Errorf("setup project identity: %w", err)
	}
	options.Root = root
	options.Env = slices.Clone(options.Env)
	if !filepath.IsAbs(options.StateDir) || !filepath.IsAbs(options.BinDir) {
		return Plan{}, fmt.Errorf("setup state directory and GOBIN must be absolute: %w", install.ErrPath)
	}
	hooks, golaw, err := resolveRouting(ctx, options, deps)
	if err != nil {
		return Plan{}, err
	}
	return planHooks(options, identity, hooks, golaw)
}

func planHooks(options Options, identity, hooks, golaw string) (Plan, error) {
	changes, err := snapshotHooks(hooks)
	if err != nil {
		return Plan{}, err
	}
	receipt := Receipt{
		Schema: ReceiptSchema, Identity: identity, Root: options.Root, Version: options.Version,
		HooksPath: hooks, Golaw: golaw, ArmedAt: options.Now,
	}
	return Plan{
		Options: options, Receipt: receipt, ReceiptPath: ReceiptPath(options.StateDir, identity), Changes: changes,
	}, nil
}

func alreadyInstalled(ctx context.Context, plan Plan, deps Dependencies) (bool, error) {
	previous, err := ReadReceipt(plan.ReceiptPath)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	want := plan.Receipt
	if previous.Identity != want.Identity || previous.Root != want.Root || previous.Version != want.Version ||
		previous.HooksPath != want.HooksPath || previous.Golaw != want.Golaw || !hooksMatch(previous, plan.Changes) {
		return false, nil
	}
	return toolsMatch(ctx, plan.Options, deps, previous.Tools)
}

func hooksMatch(receipt Receipt, changes []install.Change) bool {
	if len(receipt.Hooks) != len(changes) {
		return false
	}
	for _, change := range changes {
		artifact := install.Artifact{Path: change.Path, SHA256: install.Digest(change.Before)}
		if !change.Existed || !slices.Contains(receipt.Hooks, artifact) {
			return false
		}
	}
	return hooksExecutable(changes)
}

func hooksExecutable(changes []install.Change) bool {
	if runtime.GOOS == "windows" {
		return true
	}
	const executeBits = 0o111
	for _, change := range changes {
		info, err := os.Stat(change.Path)
		if err != nil || info.Mode().Perm()&executeBits == 0 {
			return false
		}
	}
	return true
}

func toolsMatch(ctx context.Context, options Options, deps Dependencies, recorded []ToolRecord) (bool, error) {
	tools := runner.Toolchain()
	if len(tools) != len(recorded) {
		return false, nil
	}
	for _, tool := range tools {
		path, err := verifyTool(ctx, options, deps, tool)
		if errors.Is(err, runner.ErrToolMissing) || errors.Is(err, runner.ErrToolVersion) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !slices.Contains(recorded, ToolRecord{Name: tool.Name, Version: tool.Version, Path: path}) {
			return false, nil
		}
	}
	return true, nil
}

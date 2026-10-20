package setup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/runner"
)

const lefthookTool = "lefthook"

// Apply records ownership only after installation and output both succeed.
func Apply(ctx context.Context, plan Plan, out io.Writer, deps Dependencies) error {
	if plan.Uninstall {
		return applyUninstall(ctx, plan, out, deps)
	}
	if err := checkPlan(ctx, plan, deps); err != nil {
		return err
	}
	intact, err := alreadyInstalled(ctx, plan, deps)
	if err != nil || intact {
		return err
	}
	if removeErr := removeCompletedReceipt(ctx, plan.ReceiptPath); removeErr != nil {
		return removeErr
	}
	tools, err := prepareTools(ctx, plan.Options, deps)
	if err != nil {
		return err
	}
	if err := armHooks(ctx, plan, deps); err != nil {
		return err
	}
	plan.Receipt.Tools = tools
	return finishInstall(ctx, plan, out)
}

func checkPlan(ctx context.Context, plan Plan, deps Dependencies) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	hooks, golaw, err := resolveRouting(ctx, plan.Options, deps)
	if err != nil {
		return fmt.Errorf("recheck setup routing: %w: %w", install.ErrChanged, err)
	}
	if hooks != plan.Receipt.HooksPath || golaw != plan.Receipt.Golaw {
		return fmt.Errorf("setup routing changed; plan setup again: %w", install.ErrChanged)
	}
	return checkHookInputs(plan.Changes)
}

func removeCompletedReceipt(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove incomplete setup receipt: %w", err)
	}
	return nil
}

func armHooks(ctx context.Context, plan Plan, deps Dependencies) error {
	if err := checkPlan(ctx, plan, deps); err != nil {
		return err
	}
	output, err := runCommand(ctx, plan.Options, deps, lefthookTool, []string{"install"}, runner.UserEnv(plan.Options.Env))
	if err != nil {
		return err
	}
	if output.Code != 0 {
		return fmt.Errorf("lefthook install exited %d: %s%s: %w", output.Code, output.Stdout, output.Stderr, ErrGitHooks)
	}
	return nil
}

func finishInstall(ctx context.Context, plan Plan, out io.Writer) error {
	changes, artifacts, err := completedHooks(plan.Changes)
	if err != nil {
		return err
	}
	if err := writeHookOutput(ctx, out, changes); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	plan.Receipt.Hooks = artifacts
	return WriteReceipt(plan.ReceiptPath, plan.Receipt)
}

func applyUninstall(ctx context.Context, plan Plan, out io.Writer, deps Dependencies) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if plan.Receipt.Schema == 0 && plan.ReceiptPath == "" {
		return nil
	}
	if len(plan.Receipt.Hooks) == 0 || slices.ContainsFunc(plan.Changes, func(change install.Change) bool { return !change.Remove }) {
		return fmt.Errorf("setup receipt-owned uninstall plan required: %w", install.ErrPath)
	}
	current, err := recheckUninstall(ctx, plan, deps)
	if err != nil {
		return err
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if err := install.WriteDiff(out, current.Changes); err != nil {
		return fmt.Errorf("write setup removal diff: %w", err)
	}
	return runUninstall(ctx, current, deps)
}

func runUninstall(ctx context.Context, plan Plan, deps Dependencies) error {
	if err := verifyUninstallTool(ctx, plan.Options, deps); err != nil {
		return err
	}
	if _, err := recheckUninstall(ctx, plan, deps); err != nil {
		return err
	}
	output, err := runCommand(ctx, plan.Options, deps, lefthookTool, []string{"uninstall"}, runner.UserEnv(plan.Options.Env))
	if err != nil {
		return err
	}
	if output.Code != 0 {
		return fmt.Errorf("lefthook uninstall exited %d: %s%s: %w", output.Code, output.Stdout, output.Stderr, ErrGitHooks)
	}
	return removeCompletedReceipt(ctx, plan.ReceiptPath)
}

func verifyUninstallTool(ctx context.Context, options Options, deps Dependencies) error {
	for _, tool := range runner.Toolchain() {
		if tool.Name == lefthookTool {
			_, err := verifyTool(ctx, options, deps, tool)
			return err
		}
	}
	return fmt.Errorf("lefthook pin unavailable; install a complete golaw binary: %w", runner.ErrToolMissing)
}

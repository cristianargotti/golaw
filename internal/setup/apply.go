package setup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/runner"
)

// Apply records ownership only after installation and output both succeed.
func Apply(ctx context.Context, plan Plan, out io.Writer, deps Dependencies) error {
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
	if plan.Uninstall {
		return fmt.Errorf("setup installation plan required: %w", install.ErrPath)
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
	output, err := runCommand(ctx, plan.Options, deps, "lefthook", []string{"install"}, runner.UserEnv(plan.Options.Env))
	if err != nil {
		return err
	}
	if output.Code != 0 {
		return fmt.Errorf("lefthook install exited %d: %s%s: %w", output.Code, output.Stdout, output.Stderr, install.ErrHooks)
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

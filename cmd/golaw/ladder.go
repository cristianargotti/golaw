package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
)

// affectedCommand binds interruption to the entire lifetime of its staged tree.
func (a *app) affectedCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "affected [path]",
		Short: "List packages affected by the staged index",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			set, err := a.bind().affected(ctx, rootArg(args))
			if err != nil {
				return fmt.Errorf("affected: %w", err)
			}
			return writeLines(cmd.OutOrStdout(), set.Patterns)
		},
	}
}

func (a *app) pushRangeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "push-range <remote>",
		Short: "Resolve the pushed ranges from git ref updates on stdin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ranges, err := a.bind().pushRanges(cmd.Context(), ".", args[0], a.stdin)
			if err != nil {
				return fmt.Errorf("push-range: %w", err)
			}
			return writeLines(cmd.OutOrStdout(), pushRangeLines(ranges))
		},
	}
}

// pushRangeLines keeps two fields even when git has no locally known base.
func pushRangeLines(ranges []runner.PushRange) []string {
	lines := make([]string, 0, len(ranges))
	for _, pushed := range ranges {
		if pushed.Delete {
			continue
		}
		base := pushed.Base
		if base == "" {
			base = strings.Repeat("0", len(pushed.Tip))
		}
		lines = append(lines, base+" "+pushed.Tip)
	}
	return lines
}

// lockCommand uses the separator to preserve the child's argument boundaries.
func (a *app) lockCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "lock <name> -- <command> [args...]",
		Short: "Run a command while holding a repository lock",
		Args: func(cmd *cobra.Command, args []string) error {
			if cmd.ArgsLenAtDash() != 1 || len(args) < 2 {
				return fmt.Errorf("%w: use lock <name> -- <command> [args...]", model.ErrUsage)
			}
			return validateLock(args[0], args[1:])
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			code, err := a.bind().locked(ctx, ".", args[0], args[1:], a.stdin, cmd.OutOrStdout(), cmd.ErrOrStderr())
			if err != nil {
				return fmt.Errorf("lock: %w", err)
			}
			if code != exitClean {
				return exitError{code: code}
			}
			return nil
		},
	}
}

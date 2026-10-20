package setup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/runner"
)

func resolveRouting(ctx context.Context, options Options, deps Dependencies) (string, string, error) {
	if _, err := gitDirectory(ctx, options, deps, []string{"rev-parse", "--absolute-git-dir"}); err != nil {
		return "", "", err
	}
	if err := requireLocalRouting(ctx, options, deps); err != nil {
		return "", "", err
	}
	hooks, err := hooksDirectory(ctx, options, deps)
	if err != nil {
		return "", "", err
	}
	if assetErr := requireLefthook(options.Root); assetErr != nil {
		return "", "", assetErr
	}
	golaw, err := resolveGolaw(options.Version)
	return hooks, golaw, err
}

func hooksDirectory(ctx context.Context, options Options, deps Dependencies) (string, error) {
	return gitDirectory(ctx, options, deps, []string{"rev-parse", "--path-format=absolute", "--git-path", "hooks"})
}

func runCommand(ctx context.Context, options Options, deps Dependencies, name string, args, env []string) (runner.Output, error) {
	const failure = "run %s: %w"
	if err := ctx.Err(); err != nil {
		return runner.Output{}, fmt.Errorf(failure, name, err)
	}
	run := deps.Run
	if run == nil {
		run = runner.Run
	}
	out, err := run(ctx, runner.Command{Name: name, Args: args, Dir: options.Root, Env: env})
	if ctxErr := ctx.Err(); ctxErr != nil {
		return out, fmt.Errorf(failure, name, ctxErr)
	}
	if err != nil {
		return out, fmt.Errorf(failure, name, err)
	}
	return out, nil
}

func gitDirectory(ctx context.Context, options Options, deps Dependencies, args []string) (string, error) {
	out, err := runCommand(ctx, options, deps, "git", args, options.Env)
	if err != nil {
		return "", err
	}
	if out.Code != 0 {
		return "", fmt.Errorf("%w: git %v exited %d: %s", install.ErrPath, args, out.Code, strings.TrimSpace(string(out.Stderr)))
	}
	directory := strings.TrimSuffix(string(out.Stdout), "\n")
	if !filepath.IsAbs(directory) {
		return "", fmt.Errorf("%w: git %v returned a directory without an absolute path: %q", install.ErrPath, args, directory)
	}
	return filepath.Clean(directory), nil
}

func requireLocalRouting(ctx context.Context, options Options, deps Dependencies) error {
	out, err := runCommand(ctx, options, deps, "git", []string{"config", "--local", "--get", "core.hooksPath"}, options.Env)
	if err != nil {
		return err
	}
	if out.Code == 1 && len(out.Stdout) == 0 && len(out.Stderr) == 0 {
		return nil
	}
	if out.Code == 0 {
		return fmt.Errorf("%w: local core.hooksPath is set; run git config --unset-all --local core.hooksPath", install.ErrPath)
	}
	return fmt.Errorf("%w: local core.hooksPath query exited %d: %s", install.ErrPath, out.Code, strings.TrimSpace(string(out.Stderr)))
}

func requireLefthook(root string) error {
	path := filepath.Join(root, "lefthook.yml")
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("lefthook.yml: run golaw init --apply: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: lefthook.yml must be a regular file; run golaw init --apply", install.ErrPath)
	}
	data, err := os.ReadFile(path) //nolint:gosec // The lefthook.yml of the root setup arms, a regular file.
	if err != nil {
		return fmt.Errorf("read lefthook.yml: %w", err)
	}
	return requireRoutes(data, localLefthook(root))
}

// localLefthook answers the lefthook-local.yml lefthook merges over
// lefthook.yml, nothing when the root holds no such regular file.
func localLefthook(root string) []byte {
	path := filepath.Join(root, "lefthook-local.yml")
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		return nil
	}
	data, err := os.ReadFile(path) //nolint:gosec // The local lefthook file of the root setup arms, a regular file.
	if err != nil {
		return nil // fallback: an unreadable local file adds no route
	}
	return data
}

func resolveGolaw(version string) (string, error) {
	path, err := exec.LookPath("golaw")
	if err != nil {
		return "", fmt.Errorf("%w: install the pinned golaw %s binary and put its directory on PATH: %w", runner.ErrToolMissing, version, err)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve golaw path: %w", err)
	}
	return absolute, nil
}

package setup

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
)

func prepareTools(ctx context.Context, options Options, deps Dependencies) ([]ToolRecord, error) {
	tools := runner.Toolchain()
	records := make([]ToolRecord, 0, len(tools))
	for _, tool := range tools {
		path, err := prepareTool(ctx, options, deps, tool)
		if err != nil {
			return nil, err
		}
		records = append(records, ToolRecord{Name: tool.Name, Version: tool.Version, Path: path})
	}
	return records, nil
}

func verifyTool(ctx context.Context, options Options, deps Dependencies, tool runner.Tool) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	verify := deps.Verify
	if verify == nil {
		verify = runner.Verify
	}
	path, err := verify(ctx, tool, runner.UserEnv(options.Env))
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", ctxErr
	}
	if err != nil {
		return "", fmt.Errorf("verify %s %s: %w", tool.Name, tool.Version, err)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path for %s: %w", tool.Name, err)
	}
	return absolute, nil
}

func prepareTool(ctx context.Context, options Options, deps Dependencies, tool runner.Tool) (string, error) {
	path, err := verifyTool(ctx, options, deps, tool)
	if err == nil {
		return path, nil
	}
	if !errors.Is(err, runner.ErrToolMissing) && !errors.Is(err, runner.ErrToolVersion) {
		return "", err
	}
	out, runErr := runCommand(ctx, options, deps, "go", tool.InstallArgs(), installationEnv(options))
	if runErr != nil {
		return "", fmt.Errorf("install %s %s: %w", tool.Name, tool.Version, runErr)
	}
	if out.Code != 0 {
		return "", fmt.Errorf("install %s %s: status %d, %s: %w",
			tool.Name, tool.Version, out.Code, strings.TrimSpace(string(out.Stderr)), err)
	}
	path, err = verifyTool(ctx, options, deps, tool)
	if err != nil {
		return "", diagnoseToolPath(ctx, options, deps, tool, err)
	}
	return path, nil
}

func installationEnv(options Options) []string {
	env := runner.UserEnv(options.Env)
	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(name, "GOBIN") {
			out = append(out, entry)
		}
	}
	return append(out, "GOBIN="+options.BinDir)
}

func diagnoseToolPath(ctx context.Context, options Options, deps Dependencies, tool runner.Tool, verifyErr error) error {
	if !errors.Is(verifyErr, runner.ErrToolMissing) && !errors.Is(verifyErr, runner.ErrToolVersion) {
		return verifyErr
	}
	name := tool.Name
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(options.BinDir, name)
	out, err := runCommand(ctx, options, deps, path, tool.Probe, runner.UserEnv(options.Env))
	if err != nil {
		return fmt.Errorf("verify %s in GOBIN: %w; PATH: %w", tool.Name, err, verifyErr)
	}
	if out.Code == 0 && tool.Matches(string(out.Stdout)) {
		return fmt.Errorf("put GOBIN %q first on PATH to use %s %s: %w",
			options.BinDir, tool.Name, tool.Version, verifyErr)
	}
	return fmt.Errorf("verify %s %s after installation in %q: %w",
		tool.Name, tool.Version, options.BinDir, verifyErr)
}

package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/setup"
	"github.com/cristianargotti/golaw/internal/source"
)

type setupEnvironmentError string

func (e setupEnvironmentError) Error() string { return string(e) }

func (b binding) setupOptions(ctx context.Context, path string, now time.Time) (setup.Options, error) {
	options, err := b.setupBaseOptions(ctx, path, now)
	if err != nil {
		return setup.Options{}, err
	}
	if _, err = b.load(ctx, options.Root, now); err != nil {
		if errors.Is(err, policy.ErrNotAtHead) {
			return setup.Options{}, fmt.Errorf("commit golaw.yml, then run golaw setup: %w", err)
		}
		return setup.Options{}, err
	}
	options.BinDir, err = b.setupBinDirectory(ctx, options)
	return options, err
}

func (b binding) setupBaseOptions(ctx context.Context, path string, now time.Time) (setup.Options, error) {
	if err := ctx.Err(); err != nil {
		return setup.Options{}, fmt.Errorf("setup interrupted; run golaw setup again: %w", err)
	}
	repo, err := source.Discover(ctx, path)
	if err != nil {
		return setup.Options{}, discoverFailed(path, err)
	}
	state, err := b.installState()
	if err != nil {
		return setup.Options{}, err
	}
	return setup.Options{
		Root: repo.Root, StateDir: state, Version: b.info.Version, Env: b.environ(), Now: now,
	}, nil
}

func (b binding) setupBinDirectory(ctx context.Context, options setup.Options) (string, error) {
	out, err := runner.Run(ctx, runner.Command{
		Name: "go", Args: []string{"env", "GOBIN", "GOPATH"}, Dir: options.Root, Env: runner.UserEnv(options.Env),
	})
	if err != nil {
		return "", fmt.Errorf("the go command could not run; put go on PATH, then run golaw setup again: %w", err)
	}
	if out.Code != 0 {
		return "", setupEnvironmentError(fmt.Sprintf("go env GOBIN GOPATH exited %d: %s; repair the go environment, then run golaw setup again",
			out.Code, strings.TrimSpace(string(out.Stderr))))
	}
	return b.parseSetupBinDirectory(string(out.Stdout))
}

func (b binding) parseSetupBinDirectory(output string) (string, error) {
	lines := strings.Split(strings.TrimSuffix(strings.ReplaceAll(output, "\r\n", "\n"), "\n"), "\n")
	if len(lines) != 2 {
		return "", setupEnvironmentError(fmt.Sprintf("go env GOBIN GOPATH returned %d lines instead of 2; check the Go installation, then run golaw setup again", len(lines)))
	}
	bin := lines[0]
	fromGOPATH := bin == ""
	if fromGOPATH {
		if lines[1] == "" {
			return b.setupDefaultBinDirectory()
		}
		bin = filepath.SplitList(lines[1])[0]
		if bin == "" {
			return "", setupEnvironmentError(fmt.Sprintf("the first GOPATH entry is empty in %q; set GOBIN, then run golaw setup again", lines[1]))
		}
	}
	if !filepath.IsAbs(bin) {
		return "", setupEnvironmentError(fmt.Sprintf("GOBIN or the first GOPATH entry is the relative path %q; set an absolute GOBIN, then run golaw setup again", bin))
	}
	if fromGOPATH {
		bin = filepath.Join(bin, "bin")
	}
	return bin, nil
}

func (b binding) setupDefaultBinDirectory() (string, error) {
	home, err := b.home()
	if err != nil {
		return "", fmt.Errorf("the home directory could not be resolved; set GOBIN, then run golaw setup again: %w", err)
	}
	return filepath.Join(home, "go", "bin"), nil
}

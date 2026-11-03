package main

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/thirdparty"
	"github.com/cristianargotti/golaw/internal/runner"
)

// safeFixers are the linters whose fixes change no behavior (M7.9.3 item
// 4); every other finding stays for make lint to report.
const safeFixers = "whitespace,godot,misspell,perfsprint,usestdlibvars"

// goExt is the extension of the files the fixers read.
const goExt = ".go"

// goBinary answers the go command of the binding's PATH. The environment is
// the binding's, never the process's, so a test's fake go is the one found.
func (b binding) goBinary() (string, error) {
	if found := onPath(b.environ(), "go"); found != "" {
		return found, nil
	}
	return "", fmt.Errorf("%w: go is not on PATH", thirdparty.ErrMissing)
}

// pathOf answers the last PATH of environ, the one a process would read;
// Windows spells the name in any case.
func pathOf(environ []string) string {
	value := ""
	for _, entry := range environ {
		if name, v, ok := strings.Cut(entry, "="); ok && strings.EqualFold(name, "PATH") {
			value = v
		}
	}
	return value
}

// runGenerators runs each pinned generator from the tools module in root
// and answers a notice per run; the first failure stops the run, because a
// later generator may read what the failed one should have written.
func (b binding) runGenerators(ctx context.Context, root string, gens []thirdparty.Generator) ([]string, error) {
	if len(gens) == 0 {
		return nil, nil
	}
	goBin, err := b.goBinary()
	if err != nil {
		return nil, err
	}
	env := runner.UserEnv(b.environ())
	var lines []string
	for _, g := range gens {
		bin, err := thirdparty.Resolve(ctx, goBin, root, g, env)
		if err != nil {
			return lines, fmt.Errorf("find %s: %w", g.Name, err)
		}
		if _, err := thirdparty.Exec(ctx, bin, root, g, env); err != nil {
			return lines, fmt.Errorf("generate %s: %w", g.Config, err)
		}
		lines = append(lines, "ran "+g.Name+" "+g.Config)
	}
	return lines, nil
}

// autofix runs the safe fixers of the pinned golangci-lint over the
// packages the run wrote. It loads packages, which is why only golaw gen
// runs it and never a hook (A45); the findings it leaves are not a failure
// of the run, because make lint reports them.
func (b binding) autofix(ctx context.Context, root string, written []string) []string {
	dirs := goDirs(written)
	if len(dirs) == 0 {
		return nil
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(thirdparty.ToolsModule))); err != nil {
		return nil
	}
	goBin, err := b.goBinary()
	if err != nil {
		return []string{"not fixed: golangci-lint did not run: " + err.Error()}
	}
	before := goFiles(root, dirs)
	args := []string{"tool", "-modfile=" + thirdparty.ToolsModule, linterTool, "run", "--fix", "--enable-only", safeFixers}
	for _, dir := range dirs {
		args = append(args, "./"+dir)
	}
	command := runner.Command{Name: goBin, Args: args, Dir: root, Env: runner.UserEnv(b.environ())}
	if _, err := runner.Run(ctx, command); err != nil {
		return []string{"not fixed: golangci-lint did not run: " + err.Error()}
	}
	return fixedLines(before, goFiles(root, dirs))
}

// fixedLines names, in path order, each file whose bytes the fixers changed.
func fixedLines(before, after map[string][]byte) []string {
	var lines []string
	for _, rel := range slices.Sorted(maps.Keys(after)) {
		if old, ok := before[rel]; !ok || !bytes.Equal(old, after[rel]) {
			lines = append(lines, "fixed "+rel+": golangci-lint fixers")
		}
	}
	return lines
}

// goDirs answers, sorted and once each, the directories of the Go files.
func goDirs(written []string) []string {
	var dirs []string
	for _, rel := range written {
		if strings.HasSuffix(rel, goExt) {
			dirs = append(dirs, path.Dir(rel))
		}
	}
	slices.Sort(dirs)
	return slices.Compact(dirs)
}

// goFiles reads the Go files directly inside each directory, keyed by
// slash path, so autofix names every file the fixers touched, written by
// the run or not.
func goFiles(root string, dirs []string) map[string][]byte {
	files := map[string][]byte{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), goExt) {
				continue
			}
			rel := path.Join(dir, entry.Name())
			content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) //nolint:gosec // a file the directory listing of the repository answered
			if err == nil {
				files[rel] = content
			}
		}
	}
	return files
}

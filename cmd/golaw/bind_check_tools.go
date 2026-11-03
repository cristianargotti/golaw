package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/cristianargotti/golaw/internal/check"
	"github.com/cristianargotti/golaw/internal/runner"
)

// lintTextFlags keep the linter's text report to one line per issue, the
// shape the gate reads, whatever output the repository configures.
func lintTextFlags() []string {
	return []string{"--output.text.path=stdout", "--output.text.print-issued-lines=false", "--output.text.colors=false", "--show-stats=false"}
}

// missing is the reason of a gate whose tool is not on PATH.
func missing(tool string) string { return tool + " is not on PATH" }

// launch starts one tool from the repository root and answers the Wait
// that ends it and judges its output; a tool that cannot start or that the
// budget stopped is a red gate, never a pass.
func (p *checkPlan) launch(ctx context.Context, gate, tool string, judge func(runner.Output) check.Result, args ...string) check.Wait {
	process, err := runner.Start(ctx, runner.Command{Name: tool, Args: args, Dir: p.root, Env: p.env}, nil)
	if err != nil {
		return check.Done(couldNotRun(gate, tool, err))
	}
	return func() []check.Result {
		out, err := process.Wait(ctx)
		if err != nil {
			return []check.Result{couldNotRun(gate, tool, err)}
		}
		return []check.Result{judge(out)}
	}
}

func couldNotRun(gate, tool string, err error) check.Result {
	return check.Fail(gate, "could not run "+filepath.Base(tool)+": "+err.Error())
}

// verdict maps an exit status: 0 is green, any other is red with the
// lines the tool printed.
func verdict(gate, label string) func(runner.Output) check.Result {
	return func(out runner.Output) check.Result {
		if out.Code == 0 {
			return check.Pass(gate)
		}
		return check.Fail(gate, check.Failed(label, out.Code, out.Stdout, out.Stderr)...)
	}
}

// formatGate runs golangci-lint fmt --diff over the changed Go files, as
// make fmt formats them with the repository's formatters.
func (p *checkPlan) formatGate(ctx context.Context) check.Wait {
	if reason := p.goReason(); reason != "" {
		return check.Done(check.Skip(gateFormat, reason))
	}
	files := []string{allPackages}
	if !p.scope.Whole {
		files = osPaths(p.existing(p.scope.Select(check.GoFile)))
	}
	switch {
	case len(files) == 0:
		return check.Done(check.Skip(gateFormat, "no Go file changed"))
	case p.tools.lint == "":
		return check.Done(check.Skip(gateFormat, missing(linterTool)))
	}
	return p.launch(ctx, gateFormat, p.tools.lint, unformatted, append([]string{"fmt", "--diff"}, files...)...)
}

// unformatted names the files of a diff, or what the formatter printed
// when it failed without one.
func unformatted(out runner.Output) check.Result {
	if out.Code == 0 {
		return check.Pass(gateFormat)
	}
	if lines := check.Unformatted(string(out.Stdout)); len(lines) > 0 {
		return check.Fail(gateFormat, lines...)
	}
	return verdict(gateFormat, linterTool)(out)
}

// vetGate runs go vet over the affected packages, as make vet does over
// the module.
func (p *checkPlan) vetGate(ctx context.Context) check.Wait {
	if reason := p.goReason(); reason != "" {
		return check.Done(check.Skip(gateVet, reason))
	}
	if p.tools.golang == "" {
		return check.Done(check.Skip(gateVet, missing("go")))
	}
	return p.launch(ctx, gateVet, p.tools.golang, verdict(gateVet, "go vet"), append([]string{"vet"}, p.affected.Patterns...)...)
}

// lintGate runs golangci-lint over the affected packages with the
// repository's configuration and the new code period of make lint; a
// changed configuration lints the whole module.
func (p *checkPlan) lintGate(ctx context.Context) check.Wait {
	if reason := p.goReason(); reason != "" && !p.lintConfigChanged() {
		return check.Done(check.Skip(gateLint, reason))
	}
	if p.tools.lint == "" {
		return check.Done(check.Skip(gateLint, missing(linterTool)))
	}
	args := append([]string{"run"}, lintTextFlags()...)
	if !p.scope.Whole {
		args = append(args, "--new-from-merge-base="+p.scope.Base)
	}
	patterns := p.affected.Patterns
	if p.lintConfigChanged() {
		patterns = []string{allPackages}
	}
	return p.launch(ctx, gateLint, p.tools.lint, verdict(gateLint, linterTool), append(args, patterns...)...)
}

func (p *checkPlan) lintConfigChanged() bool {
	return !p.scope.Whole && len(p.scope.Select(lintConfig)) > 0
}

// lintConfig reports whether a path is a golangci-lint configuration.
func lintConfig(name string) bool {
	return slices.Contains([]string{".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json"}, name)
}

// migrationsGate runs squawk over the changed migrations when the db
// capability is on, as make migrate-lint does over db/migrations.
func (p *checkPlan) migrationsGate(ctx context.Context) check.Wait {
	if !p.policy.Has(dbCapability) {
		return check.Done(check.Skip(gateMigrations, "the db capability is off"))
	}
	files := p.migrations()
	switch {
	case len(files) == 0:
		return check.Done(check.Skip(gateMigrations, "no migration changed"))
	case p.tools.squawk == "":
		return check.Done(check.Skip(gateMigrations, missing(squawkTool)))
	}
	return p.launch(ctx, gateMigrations, p.tools.squawk, verdict(gateMigrations, squawkTool), osPaths(files)...)
}

// migrations are the changed migrations, every one for the whole tree.
func (p *checkPlan) migrations() []string {
	if !p.scope.Whole {
		return p.existing(p.scope.Select(check.Migration))
	}
	entries, err := os.ReadDir(filepath.Join(p.root, "db", "migrations"))
	if err != nil {
		return nil
	}
	var files []string
	for _, entry := range entries {
		if name := "db/migrations/" + entry.Name(); !entry.IsDir() && check.Migration(name) {
			files = append(files, name)
		}
	}
	return files
}

// osPaths turns slash paths into the separators of the host.
func osPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, name := range paths {
		out = append(out, filepath.FromSlash(name))
	}
	return out
}

// onPath answers the program name on the PATH of environ, empty when no
// directory holds it, the way a process started with that environment
// finds it.
func onPath(environ []string, name string) string {
	windows := runtime.GOOS == "windows"
	if windows {
		name += ".exe"
	}
	for _, dir := range filepath.SplitList(pathOf(environ)) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && (windows || info.Mode().Perm()&0o111 != 0) {
			return candidate
		}
	}
	return ""
}

package lint

import (
	"context"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// HookDriver answers the one driver whose tool may run in the hook on a
// file of the language (A28): ruff, hadolint and shellcheck. Every other
// language gets the native checks alone, because its tools start too
// slowly for the event budget (M6.6.1).
func HookDriver(l surface.Language) (Driver, bool) {
	for _, d := range Drivers() {
		if d.HookArgs == nil || !slices.Contains(d.Languages, l) {
			continue
		}
		if tool, ok := toolchain.Lookup(d.Tool); ok && tool.Hook {
			return d, true
		}
	}
	return Driver{}, false
}

// HookShell answers the dialect shellcheck reads a script as: the shebang
// first, because it is what runs the script, then the extension.
func HookShell(name string, content []byte) (string, bool) {
	first, _, _ := strings.Cut(string(content), "\n")
	if rest, ok := strings.CutPrefix(strings.TrimSuffix(first, "\r"), "#!"); ok {
		shell := interpreter(strings.Fields(rest))
		return shell, shell != ""
	}
	switch path.Ext(name) {
	case ".bash":
		return "bash", true
	case ".sh":
		return "sh", true
	}
	return "", false
}

// interpreter is the shell a shebang names, through env and its flags,
// when shellcheck reads that shell.
func interpreter(words []string) string {
	if len(words) == 0 {
		return ""
	}
	name := path.Base(words[0])
	if name == "env" {
		name = ""
		for _, word := range words[1:] {
			if !strings.HasPrefix(word, "-") {
				name = path.Base(word)
				break
			}
		}
	}
	// shellcheck reads these dialects alone; a script in any other shell
	// would be judged against the wrong grammar, so it gets no tool.
	switch name {
	case "sh", "bash", "dash", "ksh":
		return name
	}
	return ""
}

// HookFile is one file a fast tool reads in the hook or the rung.
type HookFile struct {
	Path     string           // slash path relative to the repository root
	Content  []byte           // the bytes the tool reads on stdin
	Language surface.Language // the language detection settled
}

// HookEnv is where and how the fast tools run.
type HookEnv struct {
	Resolver toolchain.Resolver // resolves from receipts alone (M6.5.2)
	ToolsDir string             // the working directory of every tool
	Kit      string             // the kit version whose configuration is read
	Set      string             // model.SurfaceParity or model.SurfaceStrict
	Env      []string           // the reduced environment of M6.6.2
}

// HookOutcome is what one fast tool answered for one file.
type HookOutcome struct {
	Tool    toolchain.Tool // the pinned tool of the file, zero when Skipped
	Issues  []Issue        // the tool's issues, named by the file's path
	Missing bool           // the tool is not installed, so nothing ran
	Killed  bool           // the tool passed its budget or answered unreadably
	Skipped bool           // no fast tool reads the file
}

// hookRequest is the run of one tool on the written bytes: stdin, the tool
// directory as the working directory and the kit configuration, so no
// file of the repository configures the tool (M6.6.2).
func hookRequest(env HookEnv, d Driver, tool toolchain.Tool, exe string, file HookFile) (toolchain.Request, error) {
	var shell, dir string
	if file.Language == surface.Shell {
		shell, _ = HookShell(file.Path, file.Content)
	}
	if d.Config != nil {
		materialized, err := toolchain.Materialize(env.ToolsDir, env.Kit, env.Set, d.Config)
		if err != nil {
			return toolchain.Request{}, fmt.Errorf("%s: the kit configuration: %w", tool.Name, err)
		}
		dir = materialized
	}
	args := d.HookArgs(HookInput{Path: file.Path, Shell: shell, ConfigDir: dir})
	return toolchain.Request{Path: exe, Args: args, Stdin: file.Content, Dir: env.ToolsDir, Env: env.Env, Timeout: toolchain.HookToolBudget}, nil
}

// hookResult reads a run. An answer golaw cannot read is never clean: in
// the hook it is a file given up, a notice and never a finding.
func hookResult(tool toolchain.Tool, d Driver, file HookFile, root string, run toolchain.Execution, err error) HookOutcome {
	switch {
	case errors.Is(err, runner.ErrStart):
		return HookOutcome{Tool: tool, Missing: true}
	case err != nil, run.Killed:
		return HookOutcome{Tool: tool, Killed: true}
	}
	issues, err := Parse(d.Format, d.Tool, run.Output, root)
	if err != nil {
		return HookOutcome{Tool: tool, Killed: true}
	}
	// The tool read stdin, so whatever name it gives is the file given.
	for i := range issues {
		issues[i].Path = file.Path
	}
	return HookOutcome{Tool: tool, Issues: issues}
}

// RunHook runs the fast tool of one file on its bytes, or answers why
// none ran. A tool without a receipt starts no process (M6.5.2).
func RunHook(ctx context.Context, env HookEnv, file HookFile) HookOutcome {
	d, ok := HookDriver(file.Language)
	if !ok {
		return HookOutcome{Skipped: true}
	}
	// HookDriver answers only a driver whose tool is pinned.
	tool, _ := toolchain.Lookup(d.Tool)
	if file.Language == surface.Shell {
		if _, ok := HookShell(file.Path, file.Content); !ok {
			return HookOutcome{Skipped: true}
		}
	}
	resolved := env.Resolver.Hook(tool)
	if !resolved.Usable() {
		return HookOutcome{Tool: tool, Missing: true}
	}
	req, err := hookRequest(env, d, tool, resolved.Path, file)
	if err != nil {
		return HookOutcome{Tool: tool, Killed: true}
	}
	run, err := toolchain.Exec(ctx, req)
	return hookResult(tool, d, file, env.ToolsDir, run, err)
}

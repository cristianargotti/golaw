package main

import (
	"context"
	"go/parser"
	"go/token"
	"runtime"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// linterTool is the pin of runner's table that formats in the hook, builds
// in setup --gen and fixes in golaw gen: one name, so the three never part.
const linterTool = "golangci-lint"

// lintFmt is the pinned formatter stage of M7.9.3 item 3, or nil when setup
// --gen never built a usable golangci-lint here: the receipt alone decides,
// so finding it starts no process (M6.5.2). The stage runs from the
// repository root, where the linter reads the repository's own
// configuration and go.mod.
func (h hookBinding) lintFmt(root string) fmtStage {
	rt, ok := runner.LookupTool(linterTool)
	if !ok {
		return nil
	}
	resolver := toolchain.Resolver{Dir: h.dirs.Tools, GOOS: h.goos, GOARCH: runtime.GOARCH}
	resolved := resolver.Hook(toolchain.GoTool(rt))
	if !resolved.Usable() {
		return nil
	}
	env := toolchain.ToolEnv(h.environ())
	return func(ctx context.Context, content []byte) ([]byte, bool) {
		// --stdin reads the bytes and prints them formatted, so the hook
		// never writes a temporary file (M6.6.1).
		run, err := toolchain.Exec(ctx, toolchain.Request{
			Path: resolved.Path, Args: []string{"fmt", "--stdin"}, Stdin: content,
			Dir: root, Env: env, Timeout: h.toolTimeout(),
		})
		// A linter that failed, was killed or printed nothing leaves the
		// bytes of the in process fixes: the pipeline formats them anyway.
		if err != nil || run.Killed || run.Output.Code != 0 || len(run.Output.Stdout) == 0 {
			return nil, false
		}
		// Bytes that are no Go source (a warning on stdout) never replace
		// the agent's file: only a certain fix is written.
		if _, err := parser.ParseFile(token.NewFileSet(), "", run.Output.Stdout, parser.SkipObjectResolution); err != nil {
			return nil, false
		}
		return run.Output.Stdout, true
	}
}

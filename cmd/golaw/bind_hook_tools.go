package main

import (
	"context"
	"runtime"
	"slices"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/lint"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// tools runs the fast linters of a post event on the bytes the snapshot
// read, one file after another under the event budget (A28), so the hook
// answers within the host's limit whatever the tools do. A tool it cannot
// run is a notice, never a denial (M6.5.3).
func (h hookBinding) tools(ctx context.Context, _ hook.Repository, policy *model.EffectivePolicy, files []hook.ToolFile) hook.ToolReport {
	event, cancel := context.WithTimeout(ctx, toolchain.HookEventBudget)
	defer cancel()
	env := toolchain.ToolEnv(h.environ())
	run := lint.HookEnv{
		Resolver: toolchain.Resolver{Dir: h.dirs.Tools, GOOS: h.goos, GOARCH: runtime.GOARCH, Env: env, Run: runner.Run},
		ToolsDir: h.dirs.Tools, Kit: policy.KitVersion, Set: policy.Surface.Rules, Env: env,
	}
	human := policy.Language
	if human == "" {
		human = model.DefaultLanguage
	}
	var report hook.ToolReport
	var noticed []string
	for _, file := range files {
		// A spent event starts nothing more: the file is given up and the
		// pipeline judges it.
		if event.Err() != nil {
			report.Killed = append(report.Killed, file.Path)
			continue
		}
		out := lint.RunHook(event, run, lint.HookFile{Path: file.Path, Content: file.Content, Language: file.Language})
		switch {
		case out.Missing:
			if slices.Contains(noticed, out.Tool.Name) {
				continue
			}
			noticed = append(noticed, out.Tool.Name)
			agent, person := toolchain.MissingNotice(out.Tool, file.Path, human)
			report.Missing, report.Person = append(report.Missing, agent), append(report.Person, person)
		case out.Killed:
			report.Killed = append(report.Killed, file.Path)
		}
		for _, issue := range out.Issues {
			report.Findings = append(report.Findings, lint.ToFinding(issue))
		}
	}
	return report
}

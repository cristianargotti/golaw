package rung

import (
	"context"
	"fmt"
	"runtime"

	"github.com/cristianargotti/golaw/internal/lint"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/runner"
	detect "github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// surfaceStep runs the fast external linters over the staged files of
// their languages (M6.6.4).
const surfaceStep = "surface"

const (
	surfaceOff   = "skipped, ladder.surface_lint is false"
	surfaceEmpty = "skipped, no staged file a fast linter reads"
)

// surface runs ruff, hadolint and shellcheck on the staged bytes and
// refuses only what they find on a line the commit adds (section 7). A
// tool the machine lacks is a notice and the commit goes on (M6.5.3): the
// pipeline stays the gate.
func (p *precommit) surface(ctx context.Context) Outcome {
	if !p.ladder.SurfaceLint {
		return passed(surfaceStep, surfaceOff)
	}
	files := p.surfaceFiles()
	if len(files) == 0 {
		return passed(surfaceStep, surfaceEmpty)
	}
	// The environment git gave the hook names the home, so the tool
	// directory is the one the person's setup filled.
	dir, err := policy.ToolsDir(runtime.GOOS, func(name string) string {
		value, _ := variable(p.deps.Environ, name)
		return value
	})
	if err != nil {
		return passed(surfaceStep, "skipped, "+err.Error())
	}
	added, err := p.introduced(ctx)
	if err != nil {
		return stepFailed(surfaceStep, err)
	}
	env := p.surfaceEnv(dir)
	out := Outcome{}
	for _, file := range files {
		out = join(out, lintFile(ctx, env, file, added[file.Path], p.language))
	}
	return out
}

// surfaceFiles are the staged hand written files whose language has a
// fast linter, in target order.
func (p *precommit) surfaceFiles() []lint.HookFile {
	var files []lint.HookFile
	for _, target := range p.scan.Input.Targets {
		content, err := p.scan.Index.Read(target.Path)
		if err != nil {
			continue
		}
		info, err := detect.Detect(target.Path, content)
		if err != nil || info.Class != detect.ClassSource {
			continue
		}
		if _, ok := lint.HookDriver(info.Language); ok {
			files = append(files, lint.HookFile{Path: target.Path, Content: content, Language: info.Language})
		}
	}
	return files
}

// surfaceEnv runs the tools of one tool directory with the reduced
// environment and the kit configuration of the set in force.
func (p *precommit) surfaceEnv(dir string) lint.HookEnv {
	env := toolchain.ToolEnv(p.deps.Environ)
	return lint.HookEnv{
		Resolver: toolchain.Resolver{Dir: dir, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Env: env, Run: runner.Run},
		ToolsDir: dir, Kit: p.inForce.KitVersion, Set: p.inForce.Surface.Rules, Env: env,
	}
}

// lintFile is the outcome of one staged file. A shellcheck finding refuses
// only under the strict set: the parity set holds no shell rule (M6.4.2)
// and a shellcheck finding is feedback (M6.6.3). A finding on a line the
// commit does not touch is counted and left to the pipeline.
func lintFile(ctx context.Context, env lint.HookEnv, file lint.HookFile, added map[int]bool, human string) Outcome {
	out := lint.RunHook(ctx, env, file)
	switch {
	case out.Skipped:
		return Outcome{}
	case out.Missing:
		_, person := toolchain.MissingNotice(out.Tool, file.Path, human)
		return passed(surfaceStep, person)
	case out.Killed:
		return passed(surfaceStep, file.Path+": "+out.Tool.Name+" did not answer within its time limit; the pipeline judges it")
	}
	return issuesOutcome(file.Path, out.Issues, added, env.Set != model.SurfaceStrict)
}

// issuesOutcome refuses the issues on the lines the commit adds, except a
// shellcheck one while feedback says the set holds no shell rule, and
// counts the others in one line.
func issuesOutcome(path string, issues []lint.Issue, added map[int]bool, feedback bool) Outcome {
	result, legacy := Outcome{}, 0
	for _, issue := range issues {
		if !added[issue.Line] {
			legacy++
			continue
		}
		line := surfaceLine(lint.ToFinding(issue))
		if feedback && issue.Tool == "shellcheck" {
			result = join(result, passed(surfaceStep, line))
			continue
		}
		result = join(result, Outcome{Code: codeFindings, Lines: []string{stepLine(surfaceStep, line)}})
	}
	if legacy > 0 {
		noun := "findings"
		if legacy == 1 {
			noun = "finding"
		}
		result = join(result, passed(surfaceStep, fmt.Sprintf("%s: %d %s on lines this commit does not touch, left to the pipeline", path, legacy, noun)))
	}
	return result
}

// surfaceLine is path:line rule (Sonar key) message, the shape of the
// hook's feedback lines (M6.6.3), so one finding reads the same in both.
func surfaceLine(f model.Finding) string {
	sonar := ""
	if f.Sonar != "" {
		sonar = " (" + f.Sonar + ")"
	}
	return fmt.Sprintf("%s:%d %s%s %s", f.Path, f.Line, f.Rule, sonar, f.Message)
}

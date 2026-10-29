package main

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/source"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/surfacefix"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// The category of row (2) of M6.10 and the doctor codes of a surface tool.
const (
	m6MissingCategory = "m6-missing"
	m6ToolMissing     = "surface-tool-missing"
	m6ToolAvailable   = "surface-tool-available"
	m6Dockerfile      = "Dockerfile"
)

// m6HookFile is a new file the agent writes: its path, the corrected
// twin's file whose bytes it holds, and the tool that would lint it.
type m6HookFile struct{ path, source, tool string }

// m6HookFiles are the hook rows' files, one per fast linter the hook runs.
// Each holds the bytes of a corrected file, so only the missing tool can
// change the answer.
func m6HookFiles() []m6HookFile {
	return []m6HookFile{
		{"test/deploy/test_extra.py", "test/deploy/test_install.py", "ruff"},
		{"deploy/extra." + m6Dockerfile, m6Dockerfile, "hadolint"},
		{"scripts/extra.sh", "scripts/smoke.sh", "shellcheck"},
	}
}

// m6MissingCases are the rows of a machine without the surface tools; none
// installs them, and each runs in the scope's own data home, which holds none.
func m6MissingCases() []m6Case {
	return []m6Case{
		{name: "doctor-lines", run: (*m6Suite).m6DoctorLines},
		{name: "hook-notice", run: (*m6Suite).m6HookNotice},
		{name: "rung-notice", run: (*m6Suite).m6RungNotice},
		{name: "lint-unmeasured", run: (*m6Suite).m6LintUnmeasured},
	}
}

// m6Needed answers, for every tool a language of repo needs, whether the
// kit's own resolution finds it usable over env, the environment the
// candidate gets; a machine that holds a tool at its pin on PATH then
// expects it available and the row still proves the rest.
func m6Needed(ctx context.Context, repo string, env []string) (map[string]bool, error) {
	found, err := source.Discover(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("discover %s: %w", repo, err)
	}
	snapshot, err := source.NewWorktree(ctx, found)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", repo, err)
	}
	inventory, err := surface.Take(ctx, snapshot)
	if err != nil {
		return nil, fmt.Errorf("take the surface of %s: %w", repo, err)
	}
	dir, err := policy.ToolsDir(runtime.GOOS, func(key string) string { return environmentValue(env, key) })
	if err != nil {
		return nil, fmt.Errorf("resolve the tools directory: %w", err)
	}
	resolver := toolchain.Resolver{Dir: dir, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Env: env, Run: runner.Run}
	want := make(map[string]bool)
	tools := toolchain.ForLanguages(inventory.Languages)
	for i := range tools {
		want[tools[i].Name] = resolver.Full(ctx, tools[i]).Usable()
	}
	return want, nil
}

// m6Notice is the agent and the person sentence of tool missing at path.
func m6Notice(tool, path string) (string, string, error) {
	found, ok := toolchain.Lookup(tool)
	if !ok {
		return "", "", fmt.Errorf("the tool table pins no %s: %w", tool, ErrAcceptance)
	}
	agent, person := toolchain.MissingNotice(found, path, model.DefaultLanguage)
	return agent, person, nil
}

// m6HookBody is the bytes of the corrected twin's file f copies.
func m6HookBody(f m6HookFile) ([]byte, error) {
	files, err := surfacefix.Files(surfacefix.Corrected)
	if err != nil {
		return nil, fmt.Errorf("read the corrected twin: %w", err)
	}
	body, found := files[f.source]
	if !found {
		return nil, fmt.Errorf("the corrected twin holds no %s: %w", f.source, ErrAcceptance)
	}
	return body, nil
}

// m6DoctorLines: doctor prints one tool line per tool the red twin's
// languages need, each as the kit's own resolution finds it.
func (s *m6Suite) m6DoctorLines(ctx context.Context) error {
	repo, err := s.m6Twin(ctx, "doctor-lines", surfacefix.Red, nil)
	if err != nil {
		return err
	}
	want, err := m6Needed(ctx, repo, s.env)
	if err != nil {
		return err
	}
	got := s.m5Golaw(ctx, repo, m5DoctorCommand, m2JSONFlag)
	if stepErr := success(got); stepErr != nil {
		return fmt.Errorf("doctor over the red twin: %w", stepErr)
	}
	lines, err := m6DoctorLinesOf(got.stdout)
	if err != nil {
		return err
	}
	return m6JudgeDoctor(lines, want)
}

// m6HookNotice: the pre event allows every hook file and the post event
// allows it with the agent sentence of its missing tool.
func (s *m6Suite) m6HookNotice(ctx context.Context) error {
	repo, err := s.m6Twin(ctx, "hook-notice", surfacefix.Corrected, nil)
	if err != nil {
		return err
	}
	for _, f := range m6HookFiles() {
		if err := s.m6HookFileNotice(ctx, repo, f); err != nil {
			return fmt.Errorf("%s: %w", f.path, err)
		}
	}
	return nil
}

// m6HookFileNotice runs the pre event, writes the file as the agent would,
// then runs the post event.
func (s *m6Suite) m6HookFileNotice(ctx context.Context, repo string, f m6HookFile) error {
	body, err := m6HookBody(f)
	if err != nil {
		return err
	}
	agent, _, err := m6Notice(f.tool, f.path)
	if err != nil {
		return err
	}
	got, err := s.m6Claude(ctx, repo, eventPre, f.path, body, s.env)
	if err != nil {
		return err
	}
	if stepErr := expect(got, wantAllow); stepErr != nil {
		return fmt.Errorf("the pre event of a missing tool: %w", stepErr)
	}
	if stepErr := writeFile(filepath.Join(repo, filepath.FromSlash(f.path)), body, fileMode); stepErr != nil {
		return stepErr
	}
	got, err = s.m6Claude(ctx, repo, eventPost, f.path, body, s.env)
	if err != nil {
		return err
	}
	return m6JudgeNotice(got, agent)
}

// m6RungNotice: the armed pre-commit rung lets a staged Python file through
// with the notice of its missing tool.
func (s *m6Suite) m6RungNotice(ctx context.Context) error {
	repo, err := s.m6Twin(ctx, "rung-notice", surfacefix.Corrected, nil)
	if err != nil {
		return err
	}
	env, err := s.m5Arm(ctx, repo)
	if err != nil {
		return err
	}
	f := m6HookFiles()[0]
	body, err := m6HookBody(f)
	if err != nil {
		return err
	}
	agent, person, err := m6Notice(f.tool, f.path)
	if err != nil {
		return err
	}
	if stepErr := writeFile(filepath.Join(repo, filepath.FromSlash(f.path)), body, fileMode); stepErr != nil {
		return stepErr
	}
	if stepErr := s.gitSteps(ctx, repo, []string{gitAdd, f.path}); stepErr != nil {
		return stepErr
	}
	got := s.execute(ctx, command{program: s.binary, args: []string{commandHook, rung.PreCommit}, dir: repo, env: env})
	return m6JudgeRung(got, agent, person)
}

// m6LintUnmeasured: lint over the corrected twin exits 2 and names every
// tool it could not run.
func (s *m6Suite) m6LintUnmeasured(ctx context.Context) error {
	repo, err := s.m6Twin(ctx, "lint-unmeasured", surfacefix.Corrected, nil)
	if err != nil {
		return err
	}
	want, err := m6Needed(ctx, repo, s.env)
	if err != nil {
		return err
	}
	missing := slices.DeleteFunc(slices.Sorted(maps.Keys(want)), func(name string) bool { return want[name] })
	if len(missing) == 0 {
		return fmt.Errorf("every surface tool resolves on PATH: this row needs a machine without them: %w", ErrAcceptance)
	}
	got := s.execute(ctx, command{program: s.binary, args: []string{m6LintCommand, m6SurfaceFlag, m2JSONFlag, "."}, dir: repo, env: s.env})
	return m6JudgeUnmeasured(got, missing)
}

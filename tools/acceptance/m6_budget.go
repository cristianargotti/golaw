package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/surfacefix"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// The category of row (3) of M6.10, its sampling and its budgets: 20 warm
// runs after 2 discarded ones, a post event under 1 s and a pre event under
// 300 ms at the 95th percentile, over one file of 300 lines per hook tool.
const (
	m6BudgetCategory = "m6-budget"
	m6Runs           = 20
	m6Warmups        = 2
	m6PostBudget     = time.Second
	m6PreBudget      = 300 * time.Millisecond
	m6KilledNotice   = m6KilledMeasure + "d, " + m6KilledVerdict
	m6LongLines      = 300
)

// The two parts of the killed notice of M6.6.1, which the hook renders
// inside its gap sentence as "golaw did not measure <file>: <cause>; the
// pipeline judges it.", so the row looks for both parts, not the phrase.
const (
	m6KilledMeasure = "not measure"
	m6KilledVerdict = "the pipeline judges it"
)

// m6SleeperSource is the slow tool: it builds offline from the standard
// library alone and outlives every budget the hook keeps.
const (
	m6SleeperSource = "package main\n\nimport \"time\"\n\nfunc main() { time.Sleep(10 * time.Second) }\n"
	m6SleeperModule = "module example.test/sleeper\n\ngo 1.27\n"
	m6SleeperTool   = "ruff"
	m6DigestHex     = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

// m6BudgetCases are the rows of the hook budgets.
func m6BudgetCases() []m6Case {
	return []m6Case{
		{name: "post-p95", tools: true, run: (*m6Suite).m6PostP95},
		{name: "pre-p95", run: (*m6Suite).m6PreP95},
		{name: "slow-tool-killed", run: (*m6Suite).m6SlowTool},
	}
}

// The nearest rank of the 95th percentile, (95*n+99)/100, rounds up.
const (
	m6Percentile = 95
	m6Hundred    = 100
)

// m6P95 is the nearest-rank 95th percentile: the 19th of 20 samples.
func m6P95(samples []time.Duration) (time.Duration, error) {
	if len(samples) == 0 {
		return 0, fmt.Errorf("no sample to measure: %w", ErrAcceptance)
	}
	sorted := slices.Sorted(slices.Values(samples))
	return sorted[(m6Percentile*len(sorted)+m6Hundred-1)/m6Hundred-1], nil
}

// m6LongShape is how one long file starts and how each later line reads;
// numbered says whether the line names its number twice.
type m6LongShape struct {
	path, first, line string
	numbered          bool
}

// m6LongShapes are the long files of the languages a hook tool reads.
func m6LongShapes() map[surface.Language]m6LongShape {
	return map[surface.Language]m6LongShape{
		surface.Python:     {"test/budget/test_budget.py", `"""The hook budget fixture."""`, "VALUE_%03d = %d", true},
		surface.Dockerfile: {"deploy/budget.Dockerfile", "FROM gcr.io/distroless/static-debian13@sha256:" + m6DigestHex, `LABEL budget.%03d="%d"`, true},
		surface.Shell:      {"scripts/budget.sh", "#!/bin/sh", `echo "line %03d"`, false},
	}
}

// m6Long is the file of m6LongLines lines a hook tool of language reads,
// generated so no fixture of that size sits in the tree.
func m6Long(language surface.Language) (string, []byte, error) {
	shape, found := m6LongShapes()[language]
	if !found {
		return "", nil, fmt.Errorf("no hook tool runs on %s: %w", language, ErrAcceptance)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", shape.first)
	for i := 1; i < m6LongLines; i++ {
		if shape.numbered {
			fmt.Fprintf(&b, shape.line+"\n", i, i)
			continue
		}
		fmt.Fprintf(&b, shape.line+"\n", i)
	}
	return shape.path, []byte(b.String()), nil
}

// m6Timed runs c and answers its wall time on the run's clock.
func (s *m6Suite) m6Timed(ctx context.Context, c command) (time.Duration, result) {
	start := s.shared.now()
	got := s.execute(ctx, c)
	return s.shared.now().Sub(start), got
}

// m6Sample discards the warm-up runs, then measures m6Runs runs of c; a
// run that does not answer cleanly makes the timing meaningless.
func (s *m6Suite) m6Sample(ctx context.Context, c command) ([]time.Duration, error) {
	samples := make([]time.Duration, 0, m6Runs)
	for i := range m6Warmups + m6Runs {
		elapsed, got := s.m6Timed(ctx, c)
		if got.err != nil || got.code != 0 || got.stderr != "" {
			return nil, fmt.Errorf("run %d of %s answered %d: %q: %w", i+1, strings.Join(c.args, " "), got.code, strings.TrimSpace(got.stderr), errors.Join(ErrAcceptance, got.err))
		}
		if i >= m6Warmups {
			samples = append(samples, elapsed)
		}
	}
	return samples, nil
}

// m6Within samples one hook event on the long file of language and refuses
// a 95th percentile over budget.
func (s *m6Suite) m6Within(ctx context.Context, repo, event string, language surface.Language, env []string, budget time.Duration) error {
	path, body, err := m6Long(language)
	if err != nil {
		return err
	}
	if event == eventPost {
		if writeErr := writeFile(filepath.Join(repo, filepath.FromSlash(path)), body, fileMode); writeErr != nil {
			return writeErr
		}
	}
	c, err := s.m6ClaudeCommand(repo, event, path, body, env)
	if err != nil {
		return err
	}
	samples, err := s.m6Sample(ctx, c)
	if err != nil {
		return err
	}
	p95, err := m6P95(samples)
	if err != nil {
		return err
	}
	if p95 > budget {
		return fmt.Errorf("a %s event on %s answers at %s at the 95th percentile, the budget is %s: %w", event, language, p95, budget, ErrAcceptance)
	}
	return nil
}

// m6PostP95: a post event on each hook language, its tool installed.
func (s *m6Suite) m6PostP95(ctx context.Context) error {
	repo, err := s.m6Twin(ctx, "post-p95", surfacefix.Corrected, nil)
	if err != nil {
		return err
	}
	env := s.m6ToolEnv()
	for _, language := range []surface.Language{surface.Python, surface.Dockerfile, surface.Shell} {
		if err := s.m6Within(ctx, repo, eventPost, language, env, m6PostBudget); err != nil {
			return err
		}
	}
	return nil
}

// m6PreP95: a pre event runs the native checks alone, since the scope's
// own data home holds no tool.
func (s *m6Suite) m6PreP95(ctx context.Context) error {
	repo, err := s.m6Twin(ctx, "pre-p95", surfacefix.Corrected, nil)
	if err != nil {
		return err
	}
	return s.m6Within(ctx, repo, eventPre, surface.Python, s.env, m6PreBudget)
}

// m6Sleeper builds the slow tool at the layout path of tool and records the
// receipt the hook trusts by size and time (M6.5.2), so the hook starts it
// as the pinned tool without a PATH probe.
func (s *m6Suite) m6Sleeper(ctx context.Context, toolsDir string, tool toolchain.Tool) error {
	dir := filepath.Join(s.base, "sleeper")
	if err := writeFile(filepath.Join(dir, m2FixtureMain), []byte(m6SleeperSource), fileMode); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, moduleFile), []byte(m6SleeperModule), fileMode); err != nil {
		return err
	}
	bin := toolchain.BinPath(toolsDir, tool, runtime.GOOS, runtime.GOARCH)
	if err := os.MkdirAll(filepath.Dir(bin), dirMode); err != nil {
		return fmt.Errorf("create the slow tool directory: %w", err)
	}
	if err := success(s.execute(ctx, command{program: s.goTool, args: []string{goBuild, "-o", bin, "."}, dir: dir, env: s.env})); err != nil {
		return fmt.Errorf("build the slow tool: %w", err)
	}
	return m6Receipt(toolsDir, tool, bin)
}

// m6Receipt records the receipt of the file at bin as tool.
func m6Receipt(toolsDir string, tool toolchain.Tool, bin string) error {
	info, err := os.Stat(bin)
	if err != nil {
		return fmt.Errorf("stat the slow tool: %w", err)
	}
	body, err := readFile(bin)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	receipt := toolchain.Receipt{
		Name: tool.Name, Version: tool.Version, Path: bin, SHA256: hex.EncodeToString(sum[:]),
		Size: info.Size(), ModTime: info.ModTime(),
	}
	if err := toolchain.WriteReceipt(toolsDir, receipt); err != nil {
		return fmt.Errorf("record the slow tool: %w", err)
	}
	return nil
}

// m6SlowTool: a post event whose ruff never answers is killed at the tool
// budget and allowed with the notice that the pipeline judges the file.
func (s *m6Suite) m6SlowTool(ctx context.Context) error {
	env := m6WithData(s.env, filepath.Join(s.base, "slow-data"))
	toolsDir, err := policy.ToolsDir(runtime.GOOS, func(key string) string { return environmentValue(env, key) })
	if err != nil {
		return fmt.Errorf("resolve the slow tools directory: %w", err)
	}
	tool, found := toolchain.Lookup(m6SleeperTool)
	if !found {
		return fmt.Errorf("the tool table pins no %s: %w", m6SleeperTool, ErrAcceptance)
	}
	if buildErr := s.m6Sleeper(ctx, toolsDir, tool); buildErr != nil {
		return buildErr
	}
	repo, err := s.m6Twin(ctx, "slow-tool-killed", surfacefix.Corrected, nil)
	if err != nil {
		return err
	}
	path, body, err := m6Long(surface.Python)
	if err != nil {
		return err
	}
	if writeErr := writeFile(filepath.Join(repo, filepath.FromSlash(path)), body, fileMode); writeErr != nil {
		return writeErr
	}
	c, err := s.m6ClaudeCommand(repo, eventPost, path, body, env)
	if err != nil {
		return err
	}
	return m6JudgeKilled(s.m6Timed(ctx, c))
}

// m6JudgeKilled wants the answer after the tool budget and before the event
// budget, with a second of margin for the process start the wall time holds,
// allowing the write with the killed notice.
func m6JudgeKilled(elapsed time.Duration, got result) error {
	switch {
	case elapsed < toolchain.HookToolBudget:
		return fmt.Errorf("the post event answered in %s, before the slow tool could be killed: it did not run: %w", elapsed, ErrAcceptance)
	case elapsed >= toolchain.HookEventBudget+time.Second:
		return fmt.Errorf("the post event answered in %s, past the event budget: %w", elapsed, ErrAcceptance)
	case got.err != nil:
		return fmt.Errorf(m6RunCandidate, got.err)
	case got.code != 0 || got.stderr != "":
		return fmt.Errorf("the post event answered exit %d %q, want 0 with no stderr: %w", got.code, strings.TrimSpace(got.stderr), ErrAcceptance)
	}
	var decoded answer
	if err := json.Unmarshal([]byte(got.stdout), &decoded); err != nil {
		return fmt.Errorf("decode the post event answer: %w", err)
	}
	if decoded.Decision == decisionBlock {
		return fmt.Errorf("the slow tool blocked the write: %s: %w", decoded.Reason, ErrAcceptance)
	}
	if !strings.Contains(decoded.Hook.Additional, m6KilledMeasure) || !strings.Contains(decoded.Hook.Additional, m6KilledVerdict) {
		return fmt.Errorf("the agent context %q lacks the notice %q: %w", decoded.Hook.Additional, m6KilledNotice, ErrAcceptance)
	}
	return nil
}

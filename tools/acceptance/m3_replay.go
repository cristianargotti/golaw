package main

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/assets"
)

const (
	m3RunnerName = "Linux"
	m3Bash       = "bash"
	m3GoFlags    = "GOFLAGS"
	m3CacheFlag  = "-modcacherw"
)

// m3Expression is an Actions expression, the one construct of a run text
// or a step env that the shell cannot evaluate.
var m3Expression = regexp.MustCompile(`\$\{\{\s*([^}]*?)\s*\}\}`)

// m3RunnerFiles are the two files a runner gives each step to carry
// variables and path entries to the steps after it.
type m3RunnerFiles struct {
	env, path string
}

// m3Runner is the state a replay carries from one step to the next, as a
// runner carries it: the environment GITHUB_ENV and GITHUB_PATH grow, and
// the values of the expressions the replay evaluates.
type m3Runner struct {
	bash, repo string
	files      m3RunnerFiles
	env        []string
	vars       map[string]string
}

// m3Replay runs the run steps of a rendered CI file in order, each through
// the bash a runner uses for a run step, and stops at the first failure,
// because a runner skips every step after a failed one.
func (s *suite) m3Replay(ctx context.Context, repo string, steps []assets.RunStep) error {
	bash, err := s.lookup(m3Bash)
	if err != nil {
		return fmt.Errorf("locate bash: %w", err)
	}
	runner, err := s.m3NewRunner(bash, repo)
	if err != nil {
		return err
	}
	for _, step := range steps {
		if stepErr := s.m3Step(ctx, &runner, step); stepErr != nil {
			return fmt.Errorf("step %q of job %s: %w", m3StepLabel(step), step.Job, stepErr)
		}
		if runner.env, err = m3ApplyRunnerFiles(runner.env, runner.files); err != nil {
			return fmt.Errorf("after step %q of job %s: %w", m3StepLabel(step), step.Job, err)
		}
	}
	return nil
}

// m3NewRunner lays the variables a runner defines over the case's
// environment. A global npm install lands under the case's base, never on
// the machine that runs the harness.
func (s *suite) m3NewRunner(bash, repo string) (m3Runner, error) {
	dir := filepath.Join(s.base, "runner")
	files := m3RunnerFiles{env: filepath.Join(dir, "env"), path: filepath.Join(dir, "path")}
	for _, path := range []string{files.env, files.path} {
		if err := writeFile(path, nil, fileMode); err != nil {
			return m3Runner{}, err
		}
	}
	temp, npm := filepath.Join(s.base, "runner-temp"), filepath.Join(s.base, "npm")
	if err := os.MkdirAll(temp, dirMode); err != nil {
		return m3Runner{}, fmt.Errorf("create the runner temporary directory: %w", err)
	}
	env := withOverlay(prependPath(s.env, filepath.Join(npm, "bin")), []string{
		"GITHUB_ENV=" + files.env, "GITHUB_PATH=" + files.path, "GITHUB_WORKSPACE=" + repo,
		"RUNNER_TEMP=" + temp, "RUNNER_OS=" + m3RunnerName, "CI=true", "GITHUB_ACTIONS=true",
		"NPM_CONFIG_PREFIX=" + npm,
	})
	vars := map[string]string{
		// A fresh repository lacks the repository variable, so it is empty.
		"vars.GOLAW_ENROLLED": "", "github.workspace": repo, "runner.temp": temp, "runner.os": m3RunnerName,
	}
	return m3Runner{bash: bash, repo: repo, files: files, env: env, vars: vars}, nil
}

func (s *suite) m3Step(ctx context.Context, runner *m3Runner, step assets.RunStep) error {
	text, err := m3Expand(step.Run, runner.vars)
	if err != nil {
		return err
	}
	overlay, err := m3StepEnvironment(step.Env, runner.vars)
	if err != nil {
		return err
	}
	got := s.execute(ctx, command{
		program: runner.bash, args: []string{"--noprofile", "--norc", "-e", "-o", "pipefail", "-c", text},
		dir: runner.repo, env: withOverlay(runner.env, overlay),
	})
	if got.err != nil {
		return fmt.Errorf("execute bash: %w", got.err)
	}
	if got.code != 0 {
		return fmt.Errorf("exit %d: %s: %w", got.code, m3Tail(got), ErrAcceptance)
	}
	return nil
}

// m3StepEnvironment expands a step's env in key order, so the first
// expression the replay lacks is the same on every run. A GOFLAGS the step
// sets keeps -modcacherw, which changes no build: the module cache is
// shared by every case, and a read-only directory in it would outlive the
// run that removes its base.
func m3StepEnvironment(env, vars map[string]string) ([]string, error) {
	overlay := make([]string, 0, len(env))
	for _, key := range slices.Sorted(maps.Keys(env)) {
		value, err := m3Expand(env[key], vars)
		if err != nil {
			return nil, fmt.Errorf("env %s: %w", key, err)
		}
		if key == m3GoFlags && !slices.Contains(strings.Fields(value), m3CacheFlag) {
			value = strings.TrimSpace(value + " " + m3CacheFlag)
		}
		overlay = append(overlay, key+"="+value)
	}
	return overlay, nil
}

// m3Expand evaluates only the expressions the replay knows, so a step that
// reads a secret or a context the replay lacks fails its row instead of
// running with a value nobody chose.
func m3Expand(text string, vars map[string]string) (string, error) {
	missing := ""
	expanded := m3Expression.ReplaceAllStringFunc(text, func(expression string) string {
		name := m3Expression.FindStringSubmatch(expression)[1]
		value, found := vars[name]
		if !found && missing == "" {
			missing = name
		}
		return value
	})
	if missing != "" {
		return "", fmt.Errorf("the replay evaluates no expression %q: %w", missing, ErrAcceptance)
	}
	return expanded, nil
}

// m3ApplyRunnerFiles carries what a step wrote to the runner files into the
// environment of the steps after it, then empties both files, as a runner
// does. A multiline value is a stated limit of the replay: the template
// writes none, and reading one by half would run a step with a wrong value.
func m3ApplyRunnerFiles(env []string, files m3RunnerFiles) ([]string, error) {
	values, err := m3RunnerLines(files.env)
	if err != nil {
		return nil, err
	}
	paths, err := m3RunnerLines(files.path)
	if err != nil {
		return nil, err
	}
	out := slices.Clone(env)
	for _, line := range values {
		key, value, found := strings.Cut(line, "=")
		if !found || key == "" || strings.Contains(key, "<<") {
			return nil, fmt.Errorf("the replay reads KEY=VALUE lines of GITHUB_ENV only, not %q: %w", line, ErrAcceptance)
		}
		out = withOverlay(out, []string{key + "=" + value})
	}
	for _, dir := range paths {
		out = prependPath(out, dir)
	}
	for _, path := range []string{files.env, files.path} {
		if writeErr := writeFile(path, nil, fileMode); writeErr != nil {
			return nil, writeErr
		}
	}
	return out, nil
}

// m3RunnerLines are the lines of a runner file without their carriage
// returns and blank lines, which a runner drops too, so a step that writes
// CRLF carries the value it meant.
func m3RunnerLines(path string) ([]string, error) {
	body, err := readFile(path)
	if err != nil {
		return nil, err
	}
	var lines []string
	for line := range strings.SplitSeq(string(body), "\n") {
		if line = strings.TrimSuffix(line, "\r"); line != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

// m3StepLabel names a step as a runner's log does, by its name or by Run
// and the first line of its command, so a failure points at the step a
// person finds in the CI file.
func m3StepLabel(step assets.RunStep) string {
	if step.Name != "" {
		return step.Name
	}
	first, _, _ := strings.Cut(step.Run, "\n")
	return "Run " + first
}

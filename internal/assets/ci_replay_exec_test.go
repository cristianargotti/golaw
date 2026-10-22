package assets_test

import (
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// ciJobReplay is one job under replay: its own repository, fake tools and
// persisted state, which no other job shares.
type ciJobReplay struct {
	root  string
	bin   string
	fakes map[string]string
	state map[string]string
}

// replayCI runs every run step of every job of the rendered pipeline
// through real bash and make with fake tools, after the structural pass,
// and proves what the steps reached through the call records of the fakes.
func replayCI(t *testing.T, in ciReplayInput) error {
	t.Helper()
	jobs, err := ciPlanJobs(t, in)
	if err != nil {
		return err
	}
	attempted, total := 0, 0
	for _, job := range jobs {
		total += len(job.steps)
		ran, err := ciReplayJob(t, in, job)
		attempted += ran
		if err != nil {
			return err
		}
	}
	if attempted != total {
		return fmt.Errorf("%w: %d of %d steps attempted", errCIReplay, attempted, total)
	}
	return nil
}

// ciReplayJob gives the job a fresh repository, tool directory, export and
// path files and environment state, runs its steps in order, stops at the
// first failure and answers how many steps it attempted.
func ciReplayJob(t *testing.T, in ciReplayInput, job ciJobPlan) (int, error) {
	t.Helper()
	replay := ciNewJobReplay(t, in)
	for index, step := range job.steps {
		if ciIsGoInstall(step) && replay.state["GOTOOLCHAIN"] != "auto" {
			return index, fmt.Errorf("%w: job %s installs with go before GOTOOLCHAIN=auto is persisted", errCIReplay, job.id)
		}
		resolved, err := ciResolvedStep(step, in.Enrollment)
		if err != nil {
			return index, err
		}
		if err := ciReplayStep(t, replay.root, resolved, replay.state); err != nil {
			ciRequireStoppedBeforeInstall(t, replay, in.FailTool)
			return index + 1, fmt.Errorf("%w (%s)", err, ciStepLabel(index+1, step))
		}
	}
	return len(job.steps), ciJobCalls(t, replay, in, job)
}

// ciNewJobReplay resolves the real shell and make first, then prepares the
// repository, the fakes on a fresh PATH, the export and path files the
// hosted runner would supply and the seeded environment: the local
// toolchain the restore step must switch, no enrollment, no make flags
// inherited from the process that runs the tests, and no exit sleep for
// the fakes, which are copies of a test binary the race detector would
// otherwise hold a second each at exit.
func ciNewJobReplay(t *testing.T, in ciReplayInput) ciJobReplay {
	t.Helper()
	for _, program := range []string{"bash", "make"} {
		if _, err := exec.LookPath(program); err != nil {
			t.Fatalf("the replay needs the real %s: %v", program, err)
		}
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	temp := filepath.Join(dir, "runner-temp")
	for _, directory := range []string{bin, temp} {
		if err := os.Mkdir(directory, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	exports, paths := filepath.Join(dir, "exports"), filepath.Join(dir, "paths")
	for _, file := range []string{exports, paths} {
		if err := os.WriteFile(file, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return ciJobReplay{root: ciReplayRoot(t, in.Files), bin: bin, fakes: ciReplayTools(t, bin, in.FailTool), state: map[string]string{
		"PATH":       bin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"GITHUB_ENV": filepath.ToSlash(exports), "GITHUB_PATH": filepath.ToSlash(paths), "RUNNER_TEMP": filepath.ToSlash(temp),
		"GOTOOLCHAIN": "local", "GOLAW_ENROLLED": "", "MAKEFLAGS": "", "MFLAGS": "", "MAKELEVEL": "",
		"GORACE": "atexit_sleep_ms=0",
	}}
}

// ciReplayRoot materializes the rendered files, byte for byte, in a fresh
// repository and answers its real root; it calls neither init nor setup.
func ciReplayRoot(t *testing.T, files map[string][]byte) string {
	t.Helper()
	repo := gittest.New(t)
	for _, path := range slices.Sorted(maps.Keys(files)) {
		repo.Write(path, string(files[path]))
	}
	return repo.Root
}

// ciMakeCommands answers the recipe lines real make would run for the
// targets, expanded, through make -n; an empty target list would select
// the default target and is refused.
func ciMakeCommands(t *testing.T, root string, targets []string) ([]string, error) {
	t.Helper()
	if len(targets) == 0 {
		return nil, fmt.Errorf("%w: no make target to expand", errCIReplay)
	}
	program, err := exec.LookPath("make")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errCIReplay, err)
	}
	state := map[string]string{"MAKEFLAGS": "", "MFLAGS": "", "MAKELEVEL": "", "GOLAW_ENROLLED": ""}
	out, err := runner.Run(t.Context(), runner.Command{
		Name: program, Args: append([]string{"-n", "--no-print-directory", "--silent"}, targets...),
		Dir: root, Env: ciReplayEnvironment(gittest.Isolated(), state, nil),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errCIReplay, err)
	}
	if out.Code != 0 {
		return nil, fmt.Errorf("%w: make -n %s exited %d: %s", errCIReplay, strings.Join(targets, " "), out.Code, ciBounded(out.Stderr))
	}
	return ciLines(string(out.Stdout)), nil
}

// ciReplayStep runs one step's command text through real bash, failing on
// the first command, in the job's repository and environment, then reads
// the exports and path additions the step persisted for the steps after it.
func ciReplayStep(t *testing.T, root string, step assets.RunStep, state map[string]string) error {
	t.Helper()
	shell, err := exec.LookPath("bash")
	if err != nil {
		return fmt.Errorf("%w: %w", errCIReplay, err)
	}
	out, err := runner.Run(t.Context(), runner.Command{
		Name: shell, Args: []string{"--noprofile", "--norc", "-e", "-o", "pipefail", "-c", step.Run},
		Dir: root, Env: ciReplayEnvironment(gittest.Isolated(), state, step.Env),
	})
	if err != nil {
		return fmt.Errorf("%w: job %s, command %q: %w", errCIReplay, step.Job, step.Run, err)
	}
	if out.Code != 0 {
		return fmt.Errorf("%w: job %s, command %q exited %d: %s", errCIReplay, step.Job, step.Run, out.Code, ciBounded(out.Stderr))
	}
	exports, err := ciReadExports(state["GITHUB_ENV"])
	if err != nil {
		return err
	}
	maps.Copy(state, exports)
	additions, err := ciReadPaths(state["GITHUB_PATH"])
	if err != nil {
		return err
	}
	for _, addition := range additions {
		if !slices.Contains(strings.Split(state["PATH"], string(os.PathListSeparator)), addition) {
			state["PATH"] = addition + string(os.PathListSeparator) + state["PATH"]
		}
	}
	return nil
}

// ciRequireStoppedBeforeInstall proves, when the injected failure is the
// checksum, that the failed verification stopped its step before the
// installation and the extraction ran.
func ciRequireStoppedBeforeInstall(t *testing.T, replay ciJobReplay, fail string) {
	t.Helper()
	if fail != "sha256sum" {
		return
	}
	for _, tool := range []string{"install", "tar"} {
		if calls := runnertest.Calls(t, replay.bin, tool); len(calls) != 0 {
			t.Errorf("%s ran after the checksum failed: %v", tool, calls)
		}
	}
}

// ciJobCalls reads the call records of a successful job: every installation
// and every consumed tool reached, golaw's scan with the enrollment the
// pipeline supplied, each image download verified, and git diff last.
func ciJobCalls(t *testing.T, replay ciJobReplay, in ciReplayInput, job ciJobPlan) error {
	t.Helper()
	if err := ciInstallCalls(t, replay, job); err != nil {
		return err
	}
	for _, tool := range slices.Sorted(maps.Keys(job.consumed)) {
		// The fake go stands in for the vulngate run, so no process reaches govulncheck.
		if tool != "govulncheck" && len(runnertest.Calls(t, replay.bin, tool)) == 0 {
			return fmt.Errorf("%w: job %s never ran %s (%s)", errCIReplay, job.id, tool, replay.fakes[tool])
		}
	}
	if job.id == "ci" {
		if err := ciScanCalls(t, replay, in); err != nil {
			return err
		}
	}
	if slices.Contains(in.Caps, "image") {
		if err := ciDownloadCalls(t, replay, job); err != nil {
			return err
		}
	}
	return ciLastGitCall(t, replay, job.id)
}

// ciLastGitCall requires the job to end with git diff --exit-code.
func ciLastGitCall(t *testing.T, replay ciJobReplay, job string) error {
	t.Helper()
	git := runnertest.Calls(t, replay.bin, "git")
	if len(git) == 0 || !slices.Equal(git[len(git)-1], []string{"diff", "--exit-code"}) {
		return fmt.Errorf("%w: job %s does not end with git diff --exit-code: %v", errCIReplay, job, git)
	}
	return nil
}

package assets_test

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
)

// errCIReplay is the failure of the generated CI proof, wrapped with what
// the structural pass or the replay found.
var errCIReplay = errors.New("generated CI replay")

// ciReplayInput is one rendered repository under its capability set, the
// tool whose failure the replay injects and the enrollment value the
// pipeline would supply.
type ciReplayInput struct {
	Files      map[string][]byte
	Caps       []string
	FailTool   string
	Enrollment string
}

// ciJobPlan is what the structural pass learns about one job: its steps in
// order, the make targets its commands name, the tools it provisions and
// the tools its expanded recipes run.
type ciJobPlan struct {
	id        string
	steps     []assets.RunStep
	targets   []string
	installed map[string]bool
	consumed  map[string]bool
}

// ciActionStep is the part of an action step the structural pass reads:
// the action it uses and the inputs it passes.
type ciActionStep struct {
	Uses string         `yaml:"uses"`
	With map[string]any `yaml:"with"`
}

// checkCIReplayPlan proves the structure of the rendered pipeline against
// the rendered Makefile through real make -n: the jobs of the capability
// set, the command forms the template emits, the targets they name, the
// tools they install and the tools the recipes consume, in both directions.
func checkCIReplayPlan(t *testing.T, in ciReplayInput) error {
	t.Helper()
	_, err := ciPlanJobs(t, in)
	return err
}

// ciPlanJobs is the structural pass with its findings kept per job, so the
// replay reads the same connections it proved.
func ciPlanJobs(t *testing.T, in ciReplayInput) ([]ciJobPlan, error) {
	t.Helper()
	steps, err := assets.RunSteps(in.Files[ciAssetPath])
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errCIReplay, err)
	}
	jobs, err := ciJobPlans(steps)
	if err != nil {
		return nil, err
	}
	if err := ciExpectedJobs(jobs, in.Caps); err != nil {
		return nil, err
	}
	if err := ciSetupGo(in.Files[ciAssetPath], in.Caps); err != nil {
		return nil, err
	}
	root := ciReplayRoot(t, in.Files)
	if err := ciGateCovers(t, root, in.Caps); err != nil {
		return nil, err
	}
	if err := ciJobsConnected(t, root, jobs); err != nil {
		return nil, err
	}
	return jobs, ciExpectedTools(jobs, in.Caps)
}

// ciJobPlans groups the run steps by job in file order and reads each
// line of each step by the forms the template emits, because a form it
// does not emit is a defect of the asset, never a command to guess at.
func ciJobPlans(steps []assets.RunStep) ([]ciJobPlan, error) {
	var jobs []ciJobPlan
	for _, step := range steps {
		if len(jobs) == 0 || jobs[len(jobs)-1].id != step.Job {
			jobs = append(jobs, ciJobPlan{id: step.Job})
		}
		job := &jobs[len(jobs)-1]
		job.steps = append(job.steps, step)
		for _, line := range ciStepLines(step) {
			command, err := ciCommandOf(line)
			if err != nil {
				return nil, fmt.Errorf("%w in job %s, %s", err, step.Job, ciStepLabel(len(job.steps), step))
			}
			job.targets = append(job.targets, command.targets...)
		}
	}
	for i := range jobs {
		installed, err := ciInstalledTools(jobs[i].steps)
		if err != nil {
			return nil, fmt.Errorf("%w in job %s", err, jobs[i].id)
		}
		jobs[i].installed = installed
	}
	return jobs, nil
}

// ciExpectedJobs requires exactly the ci job and the job of each optional
// capability in force; cli adds no job.
func ciExpectedJobs(jobs []ciJobPlan, caps []string) error {
	want := []string{"ci", "surface"}
	for _, capability := range []string{"api", "db", "image", "hexagonal"} {
		if slices.Contains(caps, capability) {
			want = append(want, capability)
		}
	}
	got := make([]string, 0, len(jobs))
	for _, job := range jobs {
		got = append(got, job.id)
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		return fmt.Errorf("%w: jobs %v, want %v for %v", errCIReplay, got, want, caps)
	}
	return nil
}

// ciSetupGo reads the inputs of the Go setup of every job from the parsed
// Actions file: the module's go line with go, the explicit toolchain the
// kit needs without caching when the repository has no module.
func ciSetupGo(data []byte, caps []string) error {
	var file struct {
		Jobs map[string]struct {
			Steps []ciActionStep `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("%w: %w", errCIReplay, err)
	}
	want := map[string]any{"go-version": "1.27.0", "cache": false}
	if slices.Contains(caps, "go") {
		want = map[string]any{"go-version-file": "go.mod"}
	}
	for _, id := range slices.Sorted(maps.Keys(file.Jobs)) {
		if err := ciSetupGoJob(id, file.Jobs[id].Steps, want); err != nil {
			return err
		}
	}
	return nil
}

// ciSetupGoJob requires one Go setup in the job, with exactly the inputs
// the capability set needs.
func ciSetupGoJob(id string, steps []ciActionStep, want map[string]any) error {
	setups := 0
	for _, step := range steps {
		if !strings.HasPrefix(step.Uses, "actions/setup-go@") {
			continue
		}
		setups++
		if !maps.Equal(step.With, want) {
			return fmt.Errorf("%w: job %s sets up Go with %v, want %v", errCIReplay, id, step.With, want)
		}
	}
	if setups != 1 {
		return fmt.Errorf("%w: job %s sets up Go %d times", errCIReplay, id, setups)
	}
	return nil
}

// ciGateCovers expands ci and the required targets of the capability set
// with real make and requires every recipe line of the required targets
// inside the expansion of ci, as many times as make would run it.
func ciGateCovers(t *testing.T, root string, caps []string) error {
	t.Helper()
	gate, err := ciMakeCommands(t, root, []string{"ci"})
	if err != nil {
		return err
	}
	required, err := ciMakeCommands(t, root, policyfiles.RequiredTargets(caps))
	if err != nil {
		return err
	}
	counts := make(map[string]int, len(gate))
	for _, line := range gate {
		counts[line]++
	}
	for _, line := range required {
		counts[line]--
		if counts[line] < 0 {
			return fmt.Errorf("%w: ci does not run the required recipe %q", errCIReplay, line)
		}
	}
	return nil
}

// ciJobsConnected expands the targets of each job and requires the job to
// provision every tool its recipes run.
func ciJobsConnected(t *testing.T, root string, jobs []ciJobPlan) error {
	t.Helper()
	for i := range jobs {
		commands, err := ciMakeCommands(t, root, jobs[i].targets)
		if err != nil {
			return fmt.Errorf("%w in job %s", err, jobs[i].id)
		}
		consumed, err := ciRecipeTools(commands)
		if err != nil {
			return fmt.Errorf("%w in job %s", err, jobs[i].id)
		}
		for _, tool := range slices.Sorted(maps.Keys(consumed)) {
			if !jobs[i].installed[tool] {
				return fmt.Errorf("%w: job %s runs %s without provisioning it", errCIReplay, jobs[i].id, tool)
			}
		}
		jobs[i].consumed = consumed
	}
	return nil
}

package lint_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/hometest"
	"github.com/cristianargotti/golaw/internal/lint"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
	"github.com/cristianargotti/golaw/internal/toolchaintest"
)

// lastUser is what hadolint 2.15.1 prints for a final USER 0
// (docker:S6471), the file named as the driver passed it.
const lastUser = `[{"code":"DL3002","column":1,"file":"Dockerfile","level":"warning","line":3,"message":"Last USER should not be root"}]`

// workspace is a tree on disk, the snapshot of the same bytes and the tool
// directory of an isolated home.
type workspace struct {
	root, tools string
	snapshot    model.Snapshot
	policy      *model.EffectivePolicy
}

func newWorkspace(t *testing.T, files map[string]string) workspace {
	t.Helper()
	root := t.TempDir()
	data := make(map[string][]byte, len(files))
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		data[name] = []byte(content)
	}
	p := model.CoreDefaults()
	return workspace{root: root, tools: hometest.New(t).Tools(), snapshot: proof.NewTree(data), policy: &p}
}

func (w workspace) options(t *testing.T, set string, only ...string) lint.Options {
	t.Helper()
	scope, err := surface.ScopeOf(t.Context(), w.snapshot, w.policy)
	if err != nil {
		t.Fatal(err)
	}
	return lint.Options{
		Root: w.root, Snapshot: w.snapshot, Policy: w.policy, Scope: scope, Set: set, Only: only, Kit: "9.9.9",
		Resolver: toolchain.Resolver{Dir: w.tools, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Env: []string{"PATH=" + t.TempDir()}, Run: runner.Run},
	}
}

func outcome(t *testing.T, run *engine.Run, id string) engine.Outcome {
	t.Helper()
	for i := range run.Outcomes {
		if run.Outcomes[i].Spec.ID == id {
			return run.Outcomes[i]
		}
	}
	t.Fatalf("no outcome %s in %+v", id, run.Outcomes)
	return engine.Outcome{}
}

func driverNamed(t *testing.T, tool string) lint.Driver {
	t.Helper()
	drivers := lint.Drivers()
	for i := range drivers {
		if drivers[i].Tool == tool {
			return drivers[i]
		}
	}
	t.Fatalf("no driver for %s", tool)
	return lint.Driver{}
}

func hadolint(t *testing.T) toolchain.Tool {
	t.Helper()
	tool, ok := toolchain.Lookup("hadolint")
	if !ok {
		t.Fatal("hadolint is not pinned")
	}
	return tool
}

func TestRunLintsTheFilesOfTheScopeWithTheKitConfiguration(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t, map[string]string{"Dockerfile": "FROM scratch\nCOPY app /app\nUSER 0\n", "README.md": "# app\n"})
	path := toolchaintest.Install(t, w.tools, hadolint(t), runtime.GOOS, runtime.GOARCH, runnertest.Behavior{Stdout: lastUser, Code: 1})
	run, err := lint.Run(t.Context(), w.options(t, model.SurfaceStrict, "hadolint"))
	if err != nil {
		t.Fatal(err)
	}
	got := outcome(t, run, "lint.hadolint")
	if got.Result.Status.Kind != model.StatusMeasured || len(got.Result.Findings) != 1 || len(run.Outcomes) != 1 {
		t.Fatalf("run = %+v", run.Outcomes)
	}
	finding := got.Result.Findings[0]
	sonar, _ := lint.SonarKey("hadolint", "DL3002")
	if finding.Check != "lint.hadolint" || finding.Code != "dl3002" || finding.Path != "Dockerfile" || finding.Line != 3 || finding.Sonar != sonar || finding.Fingerprint == "" {
		t.Errorf("finding = %+v", finding)
	}
	d := driverNamed(t, "hadolint")
	config := ""
	if d.Config != nil {
		config = toolchain.ConfigDir(w.tools, "9.9.9", model.SurfaceStrict)
		if _, statErr := os.Stat(config); statErr != nil {
			t.Errorf("the kit configuration was not materialized: %v", statErr)
		}
	}
	want := d.Args(lint.Input{Root: w.root, Files: []string{"Dockerfile"}, ConfigDir: config, Set: model.SurfaceStrict, Globals: w.policy.Surface.Globals})
	if calls := runnertest.Calls(t, filepath.Dir(path), "hadolint"); len(calls) != 1 || !slices.Equal(calls[0], want) {
		t.Errorf("calls = %v, want %v", calls, want)
	}
}

func TestRunAnswersEachToolItCouldNotJudge(t *testing.T) {
	t.Parallel()
	dockerfile := map[string]string{"Dockerfile": "FROM scratch\nUSER 0\n"}
	tests := map[string]struct {
		files   map[string]string
		install *runnertest.Behavior
		only    string
		set     string
		kind    model.StatusKind
		reason  string
	}{
		"missing tool":                  {dockerfile, nil, "hadolint", model.SurfaceStrict, model.StatusUnmeasured, "golaw setup --surface"},
		"output it cannot read":         {dockerfile, &runnertest.Behavior{Stdout: "oops", Code: 1}, "hadolint", model.SurfaceStrict, model.StatusUnmeasured, "unrecognized"},
		"no file of its language":       {map[string]string{"README.md": "# a\n"}, nil, "hadolint", model.SurfaceStrict, model.StatusNotApplicable, "no dockerfile file"},
		"outside the analyzer scope":    {map[string]string{"Dockerfile": "FROM scratch\n", "sonar-project.properties": "sonar.exclusions=Dockerfile\n"}, nil, "hadolint", model.SurfaceStrict, model.StatusNotApplicable, "analyzer scope"},
		"no parity row for shell tools": {map[string]string{"scripts/a.sh": "#!/bin/sh\necho $1\n"}, nil, "shellcheck", model.SurfaceParity, model.StatusNotApplicable, "parity"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := newWorkspace(t, tc.files)
			if tc.install != nil {
				toolchaintest.Install(t, w.tools, hadolint(t), runtime.GOOS, runtime.GOARCH, *tc.install)
			}
			run, err := lint.Run(t.Context(), w.options(t, tc.set, tc.only))
			if err != nil {
				t.Fatal(err)
			}
			got := outcome(t, run, "lint."+tc.only)
			if got.Result.Status.Kind != tc.kind || !strings.Contains(got.Result.Status.Reason, tc.reason) {
				t.Errorf("status = %+v, want %v with %q", got.Result.Status, tc.kind, tc.reason)
			}
		})
	}
}

func TestRunNamesAFileTheInventoryCouldNotJudge(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t, map[string]string{"deploy/broken.yml": "AWSTemplateFormatVersion: [\n"})
	run, err := lint.Run(t.Context(), w.options(t, model.SurfaceStrict, "hadolint"))
	if err != nil {
		t.Fatal(err)
	}
	got := outcome(t, run, "lint.inventory")
	if got.Result.Status.Kind != model.StatusUnmeasured || !strings.Contains(got.Result.Status.Reason, "deploy/broken.yml") {
		t.Errorf("inventory = %+v, want Unmeasured naming the file", got.Result.Status)
	}
}

func TestRunMergesTheDriversOfOneToolIntoOneOutcome(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t, map[string]string{"README.md": "# a\n"})
	run, err := lint.Run(t.Context(), w.options(t, model.SurfaceStrict, "ruff"))
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Outcomes) != 1 || run.Outcomes[0].Spec.ID != "lint.ruff" || run.Outcomes[0].Result.Status.Kind != model.StatusNotApplicable {
		t.Errorf("outcomes = %+v, want one NotApplicable lint.ruff", run.Outcomes)
	}
}

func TestRunRefusesAnUnknownSetOrName(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t, map[string]string{"Dockerfile": "FROM scratch\n"})
	if _, err := lint.Run(t.Context(), w.options(t, "lenient")); !errors.Is(err, model.ErrUsage) || !strings.Contains(err.Error(), "parity or strict") {
		t.Errorf("unknown set = %v", err)
	}
	if _, err := lint.Run(t.Context(), w.options(t, model.SurfaceParity, "cobol")); !errors.Is(err, model.ErrUsage) || !strings.Contains(err.Error(), "cobol") {
		t.Errorf("unknown name = %v", err)
	}
	empty := w.options(t, "")
	run, err := lint.Run(t.Context(), empty)
	if err != nil || run.Mode != model.ModeScan {
		t.Errorf("the policy's set = %+v, %v", run, err)
	}
}

func TestOnlyALanguageNameNarrowsTheFilesOfAToolOfTwoLanguages(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t, map[string]string{"Dockerfile": "FROM scratch\n"})
	run, err := lint.Run(t.Context(), w.options(t, model.SurfaceStrict, string(surface.Dockerfile)))
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range run.Outcomes {
		d := driverNamed(t, strings.TrimPrefix(o.Spec.ID, "lint."))
		if !slices.Contains(d.Languages, surface.Dockerfile) {
			t.Errorf("--only dockerfile ran %s", o.Spec.ID)
		}
	}
}

func TestDriversAreSortedByTool(t *testing.T) {
	t.Parallel()
	drivers := lint.Drivers()
	tools := make([]string, 0, len(drivers))
	for i := range drivers {
		tools = append(tools, drivers[i].Tool)
		if _, ok := toolchain.Lookup(drivers[i].Tool); !ok {
			t.Errorf("driver of an unpinned tool %q", drivers[i].Tool)
		}
	}
	if len(tools) == 0 || !slices.IsSorted(tools) {
		t.Errorf("drivers = %v", tools)
	}
}

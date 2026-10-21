package policyfiles

import (
	"context"
	"path"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// The finding codes of policy.gotoolchain-local.
const (
	codeToolchainMake    = "gotoolchain-make-local"
	codeToolchainDocker  = "gotoolchain-docker-local"
	codeToolchainActions = "gotoolchain-actions-auto"
)

// The names policy.gotoolchain-local reads.
const (
	makefileName    = "Makefile"
	toolchainVar    = "GOTOOLCHAIN"
	toolchainLocal  = "local"
	actionsDir      = ".github/workflows/"
	capabilityImage = "image"
)

type gotoolchainLocal struct{}

// GotoolchainLocal is policy.gotoolchain-local: make and every golang
// build stage run the toolchain the machine holds, never one go downloads
// by itself, and each Actions job restores the automatic toolchain after
// actions/setup-go before it runs go, so go.mod decides the toolchain there.
func GotoolchainLocal() model.Check { return gotoolchainLocal{} }

func (gotoolchainLocal) Spec() model.Spec { return spec("gotoolchain-local", ruleSupply, "go") }

// Run judges the Makefile with its includes, every Dockerfile outside the
// fixtures under the image capability and every Actions file; each half
// answers its own stated limit with the file it read.
func (gotoolchainLocal) Run(_ context.Context, in model.Input) (model.Result, error) {
	var findings []model.Finding
	for _, judge := range []func(model.Input) ([]model.Finding, string, error){judgeMakeToolchain, judgeDockerToolchain, judgeActionsToolchain} {
		found, file, err := judge(in)
		if err != nil {
			return answer(file, err), nil
		}
		findings = append(findings, found...)
	}
	return model.Result{Status: model.Measured(), Scope: "the build files", Findings: findings}, nil
}

// judgeMakeToolchain finds a root Makefile that does not export the local
// toolchain, from its own lines or from a file it includes; a repository
// without a Makefile is policy.makefile's to judge, and an assignment inside
// a conditional block may change the answer under its condition.
func judgeMakeToolchain(in model.Input) ([]model.Finding, string, error) {
	_, found, err := read(in, makefileName)
	if err != nil || !found {
		return nil, makefileName, err
	}
	mk, err := readMakefile(in, makefileName)
	if err != nil {
		return nil, makefileName, err
	}
	if v, ok := mk.conditionalVar(toolchainVar); ok {
		return nil, makefileName, outside(v.file, v.line, "GOTOOLCHAIN assigned inside the conditional block opened at "+v.condition)
	}
	value, exported := mk.exported(toolchainVar)
	if strings.Contains(value, "$") {
		return nil, makefileName, outside(makefileName, 0, "GOTOOLCHAIN exported through a make expansion")
	}
	if exported && value == toolchainLocal {
		return nil, "", nil
	}
	return []model.Finding{finding(makefileName, 0, codeToolchainMake,
		"the Makefile does not export GOTOOLCHAIN=local, so a make target may run a toolchain go downloads by itself; add export GOTOOLCHAIN=local to the Makefile or a file it includes")}, "", nil
}

// judgeDockerToolchain judges, under the image capability alone, every
// Dockerfile isDockerfile selects; a fixture under testdata is no build of
// the repository.
func judgeDockerToolchain(in model.Input) ([]model.Finding, string, error) {
	if !in.Policy.Has(capabilityImage) {
		return nil, "", nil
	}
	var out []model.Finding
	for _, f := range model.Select(in.Snapshot, isDockerfile) {
		if f.Class == model.ClassFixture || !f.Class.IsText() {
			continue
		}
		found, err := dockerToolchain(in, f.Path)
		if err != nil {
			return nil, f.Path, err
		}
		out = append(out, found...)
	}
	return out, "", nil
}

// dockerToolchain judges the golang stages of one Dockerfile: a stage whose
// own last GOTOOLCHAIN is not local is a finding at that line, and a stage
// built from it inherits the value without a second finding. A stage that
// assigns nothing keeps what the golang image sets, GOTOOLCHAIN=local.
func dockerToolchain(in model.Input, file string) ([]model.Finding, error) {
	data, _, err := read(in, file)
	if err != nil {
		return nil, err
	}
	docker, err := readDockerfile(file, data)
	if err != nil {
		return nil, err
	}
	var out []model.Finding
	for i, stage := range docker.stages {
		if !golangStage(docker.stages, i) {
			continue
		}
		last, err := lastToolchain(file, stage)
		if err != nil {
			return nil, err
		}
		if last == nil {
			continue
		}
		if value := strings.Trim(last.value, `"'`); value != toolchainLocal {
			out = append(out, finding(file, last.line, codeToolchainDocker, "the golang stage of "+file+" sets GOTOOLCHAIN="+value+
				", so the build may run a toolchain go downloads by itself; set ENV GOTOOLCHAIN=local or drop the line, since the golang image keeps the local toolchain"))
		}
	}
	return out, nil
}

// golangStage answers whether a stage is built, directly or through the
// earlier stages its FROM names, from a golang image: Docker resolves a
// stage name before an image name.
func golangStage(stages []dockerStage, i int) bool {
	for {
		from := stages[i].from
		parent := slices.IndexFunc(stages[:i], func(s dockerStage) bool { return s.name != "" && strings.EqualFold(s.name, from) })
		if parent < 0 {
			return imageName(from) == "golang"
		}
		i = parent
	}
}

// lastToolchain answers the last GOTOOLCHAIN assignment of a stage, nil
// when the stage assigns none; only the last one decides, so a build
// variable an earlier line names is no limit while the last line's is.
func lastToolchain(file string, stage dockerStage) (*dockerVar, error) {
	var last *dockerVar
	for i := range stage.env {
		if stage.env[i].name == toolchainVar {
			last = &stage.env[i]
		}
	}
	if last != nil && strings.Contains(last.value, "$") {
		return nil, outside(file, last.line, "GOTOOLCHAIN set through a build variable")
	}
	return last, nil
}

// imageName answers the repository name of an image reference without its
// registry, path, tag or digest.
func imageName(ref string) string {
	ref, _, _ = strings.Cut(ref, "@")
	name := path.Base(ref)
	name, _, _ = strings.Cut(name, ":")
	return name
}

// judgeActionsToolchain finds every actions/setup-go step after which a
// step of its job runs go before the automatic toolchain is restored; a
// file the CODEOWNERS of the tree gives to an owner other than the
// catch-all one is that team's to fix, so it is read only when it would
// carry a finding.
func judgeActionsToolchain(in model.Input) ([]model.Finding, string, error) {
	files, err := readActions(in)
	if err != nil {
		return nil, actionsDir, err
	}
	var out []model.Finding
	var foreign func(string) bool
	for _, file := range files {
		var found []model.Finding
		for _, job := range file.jobs {
			found = append(found, jobToolchain(file.path, job)...)
		}
		if len(found) == 0 {
			continue
		}
		if foreign == nil {
			owned, ownersFile, ownersErr := foreignOwners(in)
			if ownersErr != nil {
				return nil, ownersFile, ownersErr
			}
			foreign = owned
		}
		if !foreign(file.path) {
			out = append(out, found...)
		}
	}
	return out, "", nil
}

// jobToolchain judges the setup-go steps of one job.
func jobToolchain(file string, job actionsJob) []model.Finding {
	var out []model.Finding
	for i, step := range job.steps {
		if !strings.HasPrefix(step.uses, "actions/setup-go@") || restoredBeforeGo(job.steps[i+1:]) {
			continue
		}
		out = append(out, finding(file, step.line, codeToolchainActions, "a step after actions/setup-go in job "+job.id+" of "+file+
			` runs go before GOTOOLCHAIN=auto is restored; add the step echo "GOTOOLCHAIN=auto" >> "$GITHUB_ENV" right after actions/setup-go`))
	}
	return out
}

// restoredBeforeGo answers whether the steps after a setup-go restore the
// automatic toolchain before the first one that runs go; a cache or a
// checkout between them changes nothing, and a job that runs no go after
// setup-go needs no restore.
func restoredBeforeGo(steps []actionsStep) bool {
	for _, step := range steps {
		if restoresAuto(step) {
			return true
		}
		if runsGo(step) {
			return false
		}
	}
	return true
}

// restoresAuto answers whether a step writes the automatic toolchain into
// the environment of the steps after it.
func restoresAuto(step actionsStep) bool {
	return strings.Contains(step.run, "GOTOOLCHAIN=auto") && strings.Contains(step.run, "GITHUB_ENV")
}

// runsGo answers whether a step may run the go command: a run text that
// holds the word go or make, or the golangci-lint action.
func runsGo(step actionsStep) bool {
	if strings.HasPrefix(step.uses, "golangci/golangci-lint-action@") {
		return true
	}
	words := strings.FieldsFunc(step.run, func(r rune) bool { return strings.ContainsRune(" \t\n;&|()", r) })
	return slices.Contains(words, "go") || slices.Contains(words, "make")
}

// foreignOwners reads the CODEOWNERS of the tree at the first of the three
// places GitHub reads it and answers which paths it gives to an owner other
// than the catch-all one; a tree without one gives none away.
func foreignOwners(in model.Input) (func(string) bool, string, error) {
	for _, file := range []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"} {
		data, found, err := read(in, file)
		if err != nil {
			return nil, file, err
		}
		if !found {
			continue
		}
		owners, err := readOwners(file, data)
		if err != nil {
			return nil, file, err
		}
		return func(p string) bool { foreign, _ := owners.foreign(p); return foreign }, "", nil
	}
	return func(string) bool { return false }, "", nil
}

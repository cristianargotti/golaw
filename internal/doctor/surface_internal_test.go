package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestAssessSurfaceSaysWhatEachToolCanJudgeHere(t *testing.T) {
	t.Parallel()
	facts := SurfaceFacts{
		Languages: []string{"javascript", "jsx", "html", "python", "shell"},
		Scope:     []string{"surface.scope.source = properties (sonar-project.properties)", "surface.scope.exclusions = **/*_test.go (sonar-project.properties)"},
		Tools: []SurfaceTool{
			{Name: "eslint", Version: "10.13.0", Languages: []string{"javascript", "jsx", "html"}, State: "runtime-missing", Detail: "the npm tools need Node 22.13 or newer on PATH"},
			{Name: "ruff", Version: "0.17.1", Languages: []string{"python"}, State: "verified", Path: "/data/golaw/tools/bin/ruff/0.17.1/darwin-arm64/ruff"},
			{Name: "shellcheck", Version: "0.11.0", Languages: []string{"shell"}, State: "path", Path: "/usr/bin/shellcheck"},
			{Name: "shfmt", Version: "3.14.1", Languages: []string{"shell"}, State: "missing"},
			{Name: "hadolint", Version: "2.15.1", Languages: []string{"dockerfile"}, State: "drift", Path: "/data/golaw/tools/bin/hadolint/2.15.1/darwin-arm64/hadolint"},
		},
		Runtimes: []SurfaceTool{{Name: "node", State: "missing", Detail: "the npm tools need Node 22.13 or newer on PATH"}},
	}
	want := []Line{
		{ID: "doctor.surface-languages", State: "ok", Code: "surface-languages", Detail: "javascript, jsx, html, python, shell"},
		{ID: "doctor.surface-scope", State: "ok", Code: "surface-scope", Detail: "surface.scope.source = properties (sonar-project.properties); surface.scope.exclusions = **/*_test.go (sonar-project.properties)"},
		{ID: "doctor.surface-tools", Key: "eslint", State: "missing", Code: "surface-tool-missing", Detail: "10.13.0: javascript, jsx and html scripts are not linted on this machine; run golaw setup --surface; the pipeline judges them"},
		{ID: "doctor.surface-tools", Key: "ruff", State: "ok", Code: "surface-tool-available", Detail: "0.17.1 /data/golaw/tools/bin/ruff/0.17.1/darwin-arm64/ruff (sha256 verified)"},
		{ID: "doctor.surface-tools", Key: "shellcheck", State: "ok", Code: "surface-tool-available", Detail: "0.11.0 /usr/bin/shellcheck (on PATH, version matches, checksum not verified)"},
		{ID: "doctor.surface-tools", Key: "shfmt", State: "missing", Code: "surface-tool-missing", Detail: "3.14.1: shell is not linted on this machine; run golaw setup --surface; the pipeline judges it"},
		{ID: "doctor.surface-tools", Key: "hadolint", State: "drift", Code: "surface-tool-drift", Detail: "2.15.1 at /data/golaw/tools/bin/hadolint/2.15.1/darwin-arm64/hadolint no longer matches its receipt: run golaw setup --surface"},
		{ID: "doctor.surface-runtimes", Key: "node", State: "missing", Code: "surface-runtime-missing", Detail: "the npm tools need Node 22.13 or newer on PATH"},
	}
	if got := AssessSurface(facts); !reflect.DeepEqual(got, want) {
		t.Errorf("AssessSurface =\n%+v\nwant\n%+v", got, want)
	}
}

func TestAssessSurfaceOfARepositoryWithoutSurfaceOrOutsideM6(t *testing.T) {
	t.Parallel()
	none := AssessSurface(SurfaceFacts{})
	if len(none) != 1 || none[0].Detail != "none" || none[0].State != "ok" {
		t.Errorf("no surface = %+v", none)
	}
	shaped := AssessSurface(SurfaceFacts{Languages: []string{"typescript"}, Shape: "package.json declares a build script"})
	want := Line{ID: "doctor.surface-shape", State: "outside", Code: "surface-shape", Detail: "package.json declares a build script: golaw lints the non-Go surface of a Go repository; the pipeline judges this project"}
	if len(shaped) != 2 || !reflect.DeepEqual(shaped[1], want) {
		t.Errorf("shape = %+v", shaped)
	}
	available := AssessSurface(SurfaceFacts{Languages: []string{"python"}, Runtimes: []SurfaceTool{{Name: "python", Version: "3.12.1", State: "verified", Path: "/usr/bin/python3"}}})
	if got := available[len(available)-1]; got.Code != "surface-runtime-available" || got.Detail != "3.12.1 /usr/bin/python3" || got.State != "ok" {
		t.Errorf("runtime = %+v", got)
	}
}

func TestNoSurfaceLineIsRed(t *testing.T) {
	t.Parallel()
	lines := AssessSurface(SurfaceFacts{
		Languages: []string{"yaml"}, Shape: "x",
		Tools:    []SurfaceTool{{Name: "yamllint", Version: "1.38.0", Languages: []string{"yaml"}, State: "runtime-missing"}},
		Runtimes: []SurfaceTool{{Name: "python", State: "missing", Detail: "the PyPI tools need Python 3.10 or newer and older than 3.15 on PATH"}},
	})
	for _, line := range lines {
		if line.State == stateRed {
			t.Errorf("red line %+v: a missing tool is never red (M6.5.3)", line)
		}
	}
}

// surfaceDiagnosticCheck proves one surface doctor code both ways: its
// fixture is the SurfaceFacts the binding collects, as JSON.
type surfaceDiagnosticCheck struct{ id, code string }

func (c surfaceDiagnosticCheck) Spec() model.Spec {
	return model.Spec{ID: c.id, Rule: "rules/5.6@1", Scope: model.ScopeTree, Owner: model.OwnerCore, Proof: c.id}
}

func (c surfaceDiagnosticCheck) Run(_ context.Context, in model.Input) (model.Result, error) {
	data, err := in.Snapshot.Read("input.json")
	if err != nil {
		return model.Result{}, fmt.Errorf("read surface diagnostic fixture: %w", err)
	}
	var facts SurfaceFacts
	if err := json.Unmarshal(data, &facts); err != nil {
		return model.Result{}, fmt.Errorf("decode surface diagnostic fixture: %w", err)
	}
	result := model.Result{Status: model.Measured(), Scope: c.id}
	for _, line := range AssessSurface(facts) {
		if line.Code == c.code {
			result.Findings = append(result.Findings, model.Finding{Code: line.Code, Message: line.Detail, Severity: model.SeverityNote})
		}
	}
	return result, nil
}

func registerSurfaceProof(t *testing.T, builder *engine.Builder) {
	t.Helper()
	for _, c := range []surfaceDiagnosticCheck{
		{"doctor.surface-tool-missing", "surface-tool-missing"},
		{"doctor.surface-tool-drift", "surface-tool-drift"},
		{"doctor.surface-runtime-missing", "surface-runtime-missing"},
		{"doctor.surface-shape", "surface-shape"},
	} {
		if err := builder.Register(c); err != nil {
			t.Fatal(err)
		}
	}
}

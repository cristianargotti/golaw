package lint

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// infraStrictTools lists the drivers whose rules only the strict set holds.
func infraStrictTools() []Driver {
	return []Driver{cfnLintDriver(), shellcheckDriver(), shfmtDriver(), yamllintDriver()}
}

func TestStrictOnlyInfraDriversReadTheirLanguages(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		d      Driver
		langs  []surface.Language
		format Format
	}{
		"cfn-lint":   {cfnLintDriver(), []surface.Language{surface.CloudFormation}, FormatCfnLint},
		"shellcheck": {shellcheckDriver(), []surface.Language{surface.Shell}, FormatShellcheck},
		"shfmt":      {shfmtDriver(), []surface.Language{surface.Shell}, FormatShfmt},
		"yamllint":   {yamllintDriver(), []surface.Language{surface.YAML}, FormatYamllint},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if tc.d.Tool != name || !slices.Equal(tc.d.Languages, tc.langs) || tc.d.Format != tc.format || tc.d.Config == nil {
				t.Errorf("%s = %+v", name, tc.d)
			}
			tool, ok := toolchain.Lookup(tc.d.Tool)
			if !ok {
				t.Fatalf("%s is not pinned in toolchain.yml", tc.d.Tool)
			}
			if tool.Hook != (tc.d.HookArgs != nil) {
				t.Errorf("%s hook pin %t, hook arguments %t", name, tool.Hook, tc.d.HookArgs != nil)
			}
		})
	}
}

func TestStrictOnlyInfraArguments(t *testing.T) {
	t.Parallel()
	root, cfg := t.TempDir(), t.TempDir()
	abs := func(rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }
	tests := map[string]struct {
		d     Driver
		files []string
		want  []string
	}{
		"cfn-lint":   {cfnLintDriver(), []string{"infra/10-base.yml"}, []string{"--format", "json", "--config-file", filepath.Join(cfg, cfnLintConfig), abs("infra/10-base.yml")}},
		"shellcheck": {shellcheckDriver(), []string{"scripts/smoke.sh"}, []string{"--norc", "--format=json1", "--severity=style", "--external-sources", "--source-path=SCRIPTDIR", abs("scripts/smoke.sh")}},
		"shfmt":      {shfmtDriver(), []string{"deploy/install.sh", "scripts/smoke.sh"}, []string{"-d", "-i", "2", "-ci", abs("deploy/install.sh"), abs("scripts/smoke.sh")}},
		"yamllint":   {yamllintDriver(), []string{"configs/app.yaml"}, []string{"--config-file", filepath.Join(cfg, yamllintConfig), "--format", "parsable", "--strict", abs("configs/app.yaml")}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			strict := Input{Root: root, Files: tc.files, ConfigDir: cfg, Set: model.SurfaceStrict}
			if got := tc.d.Args(strict); !slices.Equal(got, tc.want) {
				t.Errorf("strict %s = %q, want %q", name, got, tc.want)
			}
			parity := strict
			parity.Set = model.SurfaceParity
			if got := tc.d.Args(parity); got != nil {
				t.Errorf("%s runs under parity: %q", name, got)
			}
			empty := strict
			empty.Files = nil
			if got := tc.d.Args(empty); got != nil {
				t.Errorf("%s runs with no file: %q", name, got)
			}
		})
	}
}

// A zsh script is shell to the detector (M6.3.1), and neither shellcheck nor
// shfmt reads zsh: the lint target leaves it out as the hook does.
func TestTheShellDriversLeaveAZshScriptOut(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, content := range map[string]string{"scripts/z": "#!/usr/bin/env zsh\necho\n", "scripts/y.sh": "#!/bin/zsh -f\necho\n", "scripts/a.sh": "#!/bin/sh\necho\n"} {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	all := Input{Root: root, Files: []string{"scripts/a.sh", "scripts/y.sh", "scripts/z"}, Set: model.SurfaceStrict}
	zsh := Input{Root: root, Files: []string{"scripts/y.sh", "scripts/z"}, Set: model.SurfaceStrict}
	for _, d := range []Driver{shellcheckDriver(), shfmtDriver()} {
		got := d.Args(all)
		if !slices.Contains(got, filepath.Join(root, "scripts", "a.sh")) || slices.Contains(got, filepath.Join(root, "scripts", "y.sh")) || slices.Contains(got, filepath.Join(root, "scripts", "z")) {
			t.Errorf("%s over a sh and two zsh scripts = %q, want the sh script alone (near miss: the sh script)", d.Tool, got)
		}
		if got := d.Args(zsh); got != nil {
			t.Errorf("%s over zsh scripts alone = %q, want no run", d.Tool, got)
		}
	}
}

func TestShellcheckHookArgumentsReadStdin(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	tests := map[string]struct {
		shell string
		want  []string
	}{
		"a dialect from the shebang": {"bash", []string{"--norc", "--format=json1", "--shell=bash", "-"}},
		"no dialect":                 {"", []string{"--norc", "--format=json1", "-"}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := shellcheckDriver().HookArgs(HookInput{Path: "scripts/smoke.sh", Shell: tc.shell, ConfigDir: cfg})
			if !slices.Equal(got, tc.want) {
				t.Errorf("hook arguments = %q, want %q", got, tc.want)
			}
		})
	}
	for _, d := range []Driver{cfnLintDriver(), shfmtDriver(), yamllintDriver()} {
		if d.HookArgs != nil {
			t.Errorf("%s may run in the hook", d.Tool)
		}
	}
}

func TestStrictInfraAssetsHoldTheirProof(t *testing.T) {
	t.Parallel()
	proveInfraSet(t, strictToolsManifest, model.SurfaceStrict, infraStrictTools())
}

// TestStrictOnlyToolsStayOutOfParity proves the parity set starts none of
// them: Sonar way reports no shell, plain YAML or cfn-lint rule (survey 2.3).
func TestStrictOnlyToolsStayOutOfParity(t *testing.T) {
	t.Parallel()
	m, err := proof.LoadManifest(strictToolsManifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range m.Checks {
		fixture, err := proof.LoadFixture(filepath.Join(filepath.Dir(strictToolsManifest), e.Red))
		if err != nil {
			t.Fatal(err)
		}
		if _, ran := infraRun(t, infraEntryDriver(t, infraStrictTools(), e), model.SurfaceParity, fixture.Files); ran {
			t.Errorf("%s runs under parity", e.ID)
		}
	}
}

func TestShfmtReportsAnUnformattedScriptOnly(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		dir string
		red bool
	}{
		"four spaces": {"testdata/infra/shfmt/red", true},
		"two spaces":  {"testdata/infra/shfmt/near", false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fixture, err := proof.LoadFixture(tc.dir)
			if err != nil {
				t.Fatal(err)
			}
			issues, ran := infraRun(t, infraByTool(t, infraStrictTools(), infraShfmt), model.SurfaceStrict, fixture.Files)
			if !ran || (len(issues) > 0) != tc.red {
				t.Fatalf("shfmt ran %t with %+v, want red=%t", ran, issues, tc.red)
			}
			shfmtOnScript(t, issues)
		})
	}
}

// shfmtOnScript proves every issue is shfmt's, on the script of the trees.
func shfmtOnScript(t *testing.T, issues []Issue) {
	t.Helper()
	for _, issue := range issues {
		if issue.Tool != infraShfmt || issue.Path != "scripts/up.sh" {
			t.Errorf("issue %+v, want shfmt on scripts/up.sh", issue)
		}
	}
}

// TestInfraRunsStartsAParityToolUnderBothSets is the other side of
// infraRuns: a tool whose rules the parity set holds runs under either set
// once a file is in scope, and never without one.
func TestInfraRunsStartsAParityToolUnderBothSets(t *testing.T) {
	t.Parallel()
	files := []string{"Dockerfile"}
	for _, set := range infraSets {
		if !infraRuns(Input{Files: files, Set: set}, true) {
			t.Errorf("a parity tool stays out of %s", set)
		}
		if infraRuns(Input{Set: set}, true) {
			t.Errorf("a parity tool runs with no file under %s", set)
		}
	}
	if infraRuns(Input{Files: files, Set: "lenient"}, false) {
		t.Error("a strict-only tool runs under an unknown set")
	}
}

func TestShellcheckHookReadsTheWrittenBytes(t *testing.T) {
	t.Parallel()
	const path = "scripts/smoke.sh"
	red, err := proof.LoadFixture("testdata/infra/sc2086/red")
	if err != nil {
		t.Fatal(err)
	}
	near, err := proof.LoadFixture("testdata/infra/sc2086/near")
	if err != nil {
		t.Fatal(err)
	}
	if got := infraCodes(infraHook(t, shellcheckDriver(), model.SurfaceParity, path, "sh", red.Files[path]))["sc2086"]; got != 1 {
		t.Errorf("the hook read sc2086 %d times on the red script, want 1", got)
	}
	if got := infraHook(t, shellcheckDriver(), model.SurfaceParity, path, "sh", near.Files[path]); len(got) != 0 {
		t.Errorf("the hook reports %+v on the quoted script", got)
	}
}

// TestARepositoryConfigChangesNoStrictResult is M6.10 row (7) for the
// strict-only tools: a configuration the repository holds in the directory
// the tool starts in disables the rule, and the kit's set still reports it.
func TestARepositoryConfigChangesNoStrictResult(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		d                     Driver
		dir, name, body, code string
	}{
		"a .shellcheckrc": {shellcheckDriver(), "testdata/infra/sc2086/red", ".shellcheckrc", "disable=SC2086\n", "sc2086"},
		"a .cfnlintrc":    {cfnLintDriver(), "testdata/infra/cfn-e3002/red", ".cfnlintrc", "ignore_checks:\n  - E3002\n", "e3002"},
		"a .yamllint":     {yamllintDriver(), "testdata/infra/yaml-truthy/red", ".yamllint", "extends: default\nrules:\n  truthy: disable\n", "truthy"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fixture, err := proof.LoadFixture(tc.dir)
			if err != nil {
				t.Fatal(err)
			}
			fixture.Files[tc.name] = []byte(tc.body)
			issues, ran := infraRun(t, tc.d, model.SurfaceStrict, fixture.Files)
			if !ran || infraCodes(issues)[tc.code] == 0 {
				t.Errorf("%s hid %s: %+v", tc.name, tc.code, issues)
			}
		})
	}
}

package lint

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

func TestInfraDriversListEveryInfraToolOnce(t *testing.T) {
	t.Parallel()
	tools := make([]string, 0, len(infraDrivers()))
	for _, d := range infraDrivers() {
		tools = append(tools, d.Tool)
		tool, ok := toolchain.Lookup(d.Tool)
		if !ok {
			t.Errorf("%s is not pinned in toolchain.yml", d.Tool)
			continue
		}
		if d.Config == nil || d.Args == nil || tool.Hook != (d.HookArgs != nil) {
			t.Errorf("%s: config %t, args %t, hook pin %t, hook arguments %t", d.Tool, d.Config != nil, d.Args != nil, tool.Hook, d.HookArgs != nil)
		}
	}
	want := []string{"cfn-lint", "checkov", "hadolint", "shellcheck", "shfmt", "yamllint"}
	if !slices.Equal(tools, want) {
		t.Errorf("infra drivers = %v, want %v", tools, want)
	}
	if d := hadolintDriver(); !slices.Equal(d.Languages, []surface.Language{surface.Dockerfile}) || d.Format != FormatHadolint {
		t.Errorf("hadolint = %+v", d)
	}
	if d := checkovDriver(); !slices.Equal(d.Languages, []surface.Language{surface.Dockerfile, surface.CloudFormation}) || d.Format != FormatCheckov {
		t.Errorf("checkov = %+v", d)
	}
}

func TestHadolintArgumentsAreTheSameInBothSets(t *testing.T) {
	t.Parallel()
	root, cfg := t.TempDir(), t.TempDir()
	files := []string{"Dockerfile", "deploy/valkey.Dockerfile"}
	want := []string{
		"--config", filepath.Join(cfg, hadolintConfig), "--format", "json", "--no-color",
		filepath.Join(root, "Dockerfile"), filepath.Join(root, "deploy", "valkey.Dockerfile"),
	}
	for _, set := range infraSets {
		if got := hadolintArgs(Input{Root: root, Files: files, ConfigDir: cfg, Set: set}); !slices.Equal(got, want) {
			t.Errorf("%s = %q, want %q", set, got, want)
		}
	}
	if got := hadolintArgs(Input{Root: root, ConfigDir: cfg, Set: model.SurfaceStrict}); got != nil {
		t.Errorf("hadolint runs with no file: %q", got)
	}
	hook := []string{"--config", filepath.Join(cfg, hadolintConfig), "--format", "json", "--no-color", "-"}
	if got := hadolintHookArgs(HookInput{Path: "Dockerfile", ConfigDir: cfg}); !slices.Equal(got, hook) {
		t.Errorf("hook = %q, want %q", got, hook)
	}
}

// TestCheckovArgumentsStayOfflineAndLeaveDockerfilesToStrict: the parity
// checks are CloudFormation ones, so a Dockerfile is passed only under
// strict, and a parity run with no template starts nothing.
func TestCheckovArgumentsStayOfflineAndLeaveDockerfilesToStrict(t *testing.T) {
	t.Parallel()
	root, cfg := t.TempDir(), t.TempDir()
	files := []string{"Dockerfile", "infra/10-base.yml"}
	flags := []string{"--config-file", filepath.Join(cfg, checkovConfig), "--output", "json", "--quiet", "--compact", "--skip-download"}
	template := []string{"--file", filepath.Join(root, "infra", "10-base.yml")}
	strict := append(append(slices.Clone(flags), "--file", filepath.Join(root, "Dockerfile")), template...)
	if got := checkovArgs(Input{Root: root, Files: files, ConfigDir: cfg, Set: model.SurfaceStrict}); !slices.Equal(got, strict) {
		t.Errorf("strict = %q, want %q", got, strict)
	}
	parity := append(slices.Clone(flags), template...)
	if got := checkovArgs(Input{Root: root, Files: files, ConfigDir: cfg, Set: model.SurfaceParity}); !slices.Equal(got, parity) {
		t.Errorf("parity = %q, want %q", got, parity)
	}
	if got := checkovArgs(Input{Root: root, Files: []string{"Dockerfile"}, ConfigDir: cfg, Set: model.SurfaceParity}); got != nil {
		t.Errorf("parity runs checkov on a Dockerfile alone: %q", got)
	}
	if checkovDriver().HookArgs != nil {
		t.Error("checkov may run in the hook")
	}
}

func TestHadolintHookReadsTheWrittenBytes(t *testing.T) {
	t.Parallel()
	red, err := proof.LoadFixture("testdata/infra/dl3002/red")
	if err != nil {
		t.Fatal(err)
	}
	near, err := proof.LoadFixture("testdata/infra/dl3002/near")
	if err != nil {
		t.Fatal(err)
	}
	for _, set := range infraSets {
		if got := infraCodes(infraHook(t, hadolintDriver(), set, "Dockerfile", "", red.Files["Dockerfile"]))["dl3002"]; got != 1 {
			t.Errorf("%s hook read dl3002 %d times, want 1", set, got)
		}
		if got := infraHook(t, hadolintDriver(), set, "Dockerfile", "", near.Files["Dockerfile"]); len(got) != 0 {
			t.Errorf("%s hook reports %+v on the numeric user", set, got)
		}
	}
}

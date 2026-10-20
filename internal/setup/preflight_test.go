package setup_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/setup"
)

// routingCase is one routing setup refuses before arming: the change made
// to the fixture and the fix the refusal names.
type routingCase struct {
	name, want string
	change     func(*testing.T, setup.Options, *setupProbe)
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupRefusesKnownBrokenRoutingBeforeArming(t *testing.T) {
	for _, tc := range []routingCase{
		{"local hooks path", "git config --unset-all --local core.hooksPath", localHooksPath},
		{"missing asset", "golaw init --apply", missingAsset},
		{"GOBIN shadowed", "GOBIN", shadowedBin},
		{"golaw missing", "golaw", missingGolaw},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options, deps, probe := setupFixture(t)
			tc.change(t, options, probe)
			var out bytes.Buffer
			plan, err := setup.PlanInstall(t.Context(), options, deps)
			if err == nil {
				err = setup.Apply(t.Context(), plan, &out, deps)
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("refusal = %v, want %q", err, tc.want)
			}
			requireNotArmed(t, probe, tc.name, err)
		})
	}
}

// localHooksPath routes the hooks of the fixture through core.hooksPath.
func localHooksPath(t *testing.T, _ setup.Options, probe *setupProbe) {
	t.Helper()
	probe.repo.Git("config", "--local", "core.hooksPath", "custom-hooks")
}

// missingAsset removes the lefthook asset init writes.
func missingAsset(t *testing.T, options setup.Options, _ *setupProbe) {
	t.Helper()
	if err := os.Remove(filepath.Join(options.Root, "lefthook.yml")); err != nil {
		t.Fatal(err)
	}
}

// shadowedBin pins every tool while another lefthook comes first on PATH.
func shadowedBin(t *testing.T, _ setup.Options, probe *setupProbe) {
	t.Helper()
	probe.shadow = true
	for _, tool := range runner.Toolchain() {
		probe.ready[tool.Name] = true
	}
}

// missingGolaw removes the golaw executable from PATH.
func missingGolaw(t *testing.T, options setup.Options, probe *setupProbe) {
	t.Helper()
	if err := os.Remove(probe.golaw); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", options.BinDir)
}

// requireNotArmed requires a refused setup to arm nothing, to install
// nothing before its preflight, and to name PATH in the fix of a PATH
// refusal.
func requireNotArmed(t *testing.T, probe *setupProbe, name string, err error) {
	t.Helper()
	for _, call := range probe.calls {
		program := strings.TrimSuffix(filepath.Base(call.Name), ".exe")
		if program == "lefthook" && len(call.Args) == 1 && call.Args[0] == "install" {
			t.Fatalf("armed after refusal: %+v", call)
		}
		if name != "GOBIN shadowed" && program == "go" {
			t.Fatalf("installed before preflight: %+v", call)
		}
	}
	if (name == "GOBIN shadowed" || name == "golaw missing") &&
		!strings.Contains(err.Error(), "PATH") {
		t.Fatalf("fix lacks PATH: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(probe.hooks, "pre-push")); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("hook exists after refusal: %v", statErr)
	}
}

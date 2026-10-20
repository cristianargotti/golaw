package setup_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/setup"
)

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupRefusesEmptyLocalHookRouting(t *testing.T) {
	options, deps, probe := setupFixture(t)
	probe.repo.Git("config", "--local", "core.hooksPath", "")
	_, err := setup.PlanInstall(t.Context(), options, deps)
	if err == nil || !strings.Contains(err.Error(), "git config --unset-all --local core.hooksPath") {
		t.Fatalf("configured empty routing = %v", err)
	}
	setupSafetyNoInstalls(t, probe)
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupDoesNotTreatGitDiagnosticAsAbsentConfig(t *testing.T) {
	for _, stream := range []string{"stdout", "stderr"} {
		t.Run(stream, func(t *testing.T) {
			options, deps, probe := setupFixture(t)
			run, observed := deps.Run, false
			deps.Run = func(ctx context.Context, command runner.Command) (runner.Output, error) {
				if slices.Contains(command.Args, "core.hooksPath") && slices.Contains(command.Args, "--get") {
					observed = true
					out := runner.Output{Code: 1, Stderr: []byte("configuration unavailable\n")}
					if stream == "stdout" {
						out.Stdout, out.Stderr = out.Stderr, nil
					}
					return out, nil
				}
				return run(ctx, command)
			}
			_, err := setup.PlanInstall(t.Context(), options, deps)
			if !observed || err == nil {
				t.Fatalf("diagnostic config response: queried=%v, error=%v", observed, err)
			}
			setupSafetyNoInstalls(t, probe)
		})
	}
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupRefusesChangesAfterPlanning(t *testing.T) {
	for _, changed := range []string{"hook", "routing"} {
		t.Run(changed, func(t *testing.T) {
			setupSafetyChanged(t, changed)
		})
	}
}

func setupSafetyChanged(t *testing.T, changed string) {
	t.Helper()
	options, deps, probe := setupFixture(t)
	plan, err := setup.PlanInstall(t.Context(), options, deps)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(probe.hooks, "pre-commit")
	if changed == "hook" {
		setupWrite(t, path, foreignHook)
	} else {
		setupSafetyMoveRouting(&deps, filepath.Join(options.Root, "other-hooks"))
	}
	probe.calls = nil
	if err := setup.Apply(t.Context(), plan, io.Discard, deps); !errors.Is(err, install.ErrChanged) {
		t.Fatalf("changed %s = %v", changed, err)
	}
	setupSafetyNoInstalls(t, probe)
	if changed == "hook" {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != foreignHook {
			t.Fatalf("changed hook was overwritten: %q, %v", data, err)
		}
	}
}

func setupSafetyMoveRouting(deps *setup.Dependencies, directory string) {
	run := deps.Run
	deps.Run = func(ctx context.Context, command runner.Command) (runner.Output, error) {
		if slices.Contains(command.Args, "--git-path") && slices.Contains(command.Args, "hooks") {
			return runner.Output{Stdout: []byte(directory + "\n")}, nil
		}
		return run(ctx, command)
	}
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupRepairsMismatchedReceiptEvidence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*setup.Receipt)
	}{
		{"identity", func(r *setup.Receipt) { r.Identity = "another-project" }},
		{"root", func(r *setup.Receipt) { r.Root = filepath.Join(r.Root, "other-root") }},
		{"version", func(r *setup.Receipt) { r.Version = "0.0.1" }},
		{"golaw", func(r *setup.Receipt) { r.Golaw = filepath.Join(r.Root, "other-golaw") }},
		{"hook routing", setupSafetyOtherHookRoute},
		{"tool name", func(r *setup.Receipt) { r.Tools[0].Name = "other-tool" }},
		{"tool pin", func(r *setup.Receipt) { r.Tools[0].Version = "0.0.1" }},
		{"tool path", func(r *setup.Receipt) { r.Tools[0].Path = filepath.Join(r.Root, "other-tool") }},
		{"missing tool", func(r *setup.Receipt) { r.Tools = r.Tools[1:] }},
		{"missing hook record", func(r *setup.Receipt) { r.Hooks = r.Hooks[1:] }},
		{"hook hash", func(r *setup.Receipt) { r.Hooks[0].SHA256 = install.Digest([]byte("different\n")) }},
		{"missing hook file", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupSafetyRepair(t, tc.change)
		})
	}
}

func setupSafetyRepair(t *testing.T, change func(*setup.Receipt)) {
	t.Helper()
	options, deps, probe := setupFixture(t)
	installed := setupSafetyArm(t, options, deps)
	expected, err := setup.ReadReceipt(installed.ReceiptPath)
	if err != nil {
		t.Fatal(err)
	}
	modified := expected
	modified.Tools, modified.Hooks = slices.Clone(expected.Tools), slices.Clone(expected.Hooks)
	setupSafetyModifyEvidence(t, installed.ReceiptPath, &modified, change)
	options.Now = options.Now.Add(time.Hour)
	expected.ArmedAt = options.Now
	probe.calls = nil
	setupSafetyArm(t, options, deps)
	got, err := setup.ReadReceipt(installed.ReceiptPath)
	if err != nil || !reflect.DeepEqual(got, expected) {
		t.Fatalf("rearmed receipt = %+v, %v; want %+v", got, err, expected)
	}
	if !slices.ContainsFunc(probe.calls, func(command runner.Command) bool {
		return setupSafetyInstall(command) && strings.TrimSuffix(filepath.Base(command.Name), ".exe") == "lefthook"
	}) {
		t.Fatal("mismatched receipt was accepted without arming hooks")
	}
}

func setupSafetyModifyEvidence(t *testing.T, path string, receipt *setup.Receipt, change func(*setup.Receipt)) {
	t.Helper()
	if change == nil {
		if err := os.Remove(receipt.Hooks[0].Path); err != nil {
			t.Fatal(err)
		}
		return
	}
	change(receipt)
	if err := setup.WriteReceipt(path, *receipt); err != nil {
		t.Fatal(err)
	}
}

func setupSafetyOtherHookRoute(receipt *setup.Receipt) {
	receipt.HooksPath = filepath.Join(receipt.Root, "other-hooks")
	for index := range receipt.Hooks {
		receipt.Hooks[index].Path = filepath.Join(receipt.HooksPath, filepath.Base(receipt.Hooks[index].Path))
	}
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupKeepsMalformedReceiptForInspection(t *testing.T) {
	options, deps, probe := setupFixture(t)
	installed := setupSafetyArm(t, options, deps)
	if err := os.WriteFile(installed.ReceiptPath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	probe.calls = nil
	plan, err := setup.PlanInstall(t.Context(), options, deps)
	if err == nil {
		err = setup.Apply(t.Context(), plan, io.Discard, deps)
	}
	if !errors.Is(err, setup.ErrReceipt) {
		t.Fatalf("malformed receipt = %v", err)
	}
	setupSafetyNoInstalls(t, probe)
	data, readErr := os.ReadFile(installed.ReceiptPath)
	if readErr != nil || string(data) != "{" {
		t.Fatalf("malformed receipt changed: %q, %v", data, readErr)
	}
}

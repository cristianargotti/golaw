package setup_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/setup"
)

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestUninstallRechecksOwnershipBeforeMutation(t *testing.T) {
	for _, moment := range []string{"before apply", "during verify", "during output"} {
		for _, changed := range []string{"receipt", "hook", "routing"} {
			t.Run(moment+"/"+changed, func(t *testing.T) {
				requireUninstallRecheck(t, moment, changed)
			})
		}
	}
}

func requireUninstallRecheck(t *testing.T, moment, changed string) {
	t.Helper()
	options, deps, probe := setupFixture(t)
	receiptPath, before := armedReceipt(t, options, deps)
	plan, err := setup.PlanUninstall(t.Context(), options, deps)
	if err != nil {
		t.Fatal(err)
	}
	mutated := false
	mutate := func() {
		if !mutated {
			mutated = true
			changeUninstallEvidence(t, changed, plan, probe)
			before = readUninstallEvidence(t, receiptPath)
		}
	}
	out := io.Discard
	switch moment {
	case "before apply":
		mutate()
	case "during verify":
		deps.Verify = func(ctx context.Context, tool runner.Tool, env []string) (string, error) {
			mutate()
			return probe.verify(ctx, tool, env)
		}
	case "during output":
		out = uninstallChangeWriter{change: mutate}
	}
	probe.calls = nil
	if err := setup.Apply(t.Context(), plan, out, deps); !errors.Is(err, install.ErrChanged) {
		t.Fatalf("changed %s %s = %v", changed, moment, err)
	}
	if !mutated {
		t.Fatalf("change %s never ran", moment)
	}
	requireUninstallRetained(t, probe, receiptPath, before, changed == "hook")
	requireNoUninstallCalls(t, probe)
}

func changeUninstallEvidence(t *testing.T, kind string, plan setup.Plan, probe *setupProbe) {
	t.Helper()
	switch kind {
	case "receipt":
		receipt, err := setup.ReadReceipt(plan.ReceiptPath)
		if err != nil {
			t.Fatal(err)
		}
		receipt.Version = "0.2.1"
		if err := setup.WriteReceipt(plan.ReceiptPath, receipt); err != nil {
			t.Fatal(err)
		}
	case "hook":
		setupWrite(t, filepath.Join(probe.hooks, "pre-commit"), "edited after planning\n")
	case "routing":
		probe.repo.Git("config", "--local", "core.hooksPath", "changed-hooks")
	default:
		t.Fatalf("unknown evidence %q", kind)
	}
}

type uninstallChangeWriter struct{ change func() }

func (w uninstallChangeWriter) Write(data []byte) (int, error) {
	w.change()
	return len(data), nil
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestUninstallPreservesDependencyErrorsAndCancellation(t *testing.T) {
	for _, mode := range []string{"verify error", "verify cancellation", "run cancellation"} {
		t.Run(mode, func(t *testing.T) {
			options, deps, probe := setupFixture(t)
			receiptPath, before := armedReceipt(t, options, deps)
			plan, err := setup.PlanUninstall(t.Context(), options, deps)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)
			called := false
			deps, want := uninstallFailureDependencies(mode, deps, probe, cancel, &called)
			probe.calls = nil
			if err := setup.Apply(ctx, plan, io.Discard, deps); !errors.Is(err, want) {
				t.Fatalf("%s = %v, want %v", mode, err, want)
			}
			if !called {
				t.Fatalf("%s dependency never ran", mode)
			}
			requireUninstallRetained(t, probe, receiptPath, before, false)
			if mode != "run cancellation" {
				requireNoUninstallCalls(t, probe)
			}
		})
	}
}

func uninstallFailureDependencies(
	mode string, deps setup.Dependencies, probe *setupProbe, cancel context.CancelFunc, called *bool,
) (setup.Dependencies, error) {
	if mode == "run cancellation" {
		deps.Run = func(ctx context.Context, command runner.Command) (runner.Output, error) {
			if len(command.Args) == 1 && command.Args[0] == "uninstall" {
				*called = true
				probe.calls = append(probe.calls, command)
				cancel()
				return runner.Output{}, nil
			}
			return probe.run(ctx, command)
		}
		return deps, context.Canceled
	}
	deps.Verify = func(ctx context.Context, tool runner.Tool, env []string) (string, error) {
		*called = true
		if mode == "verify error" {
			return "", io.ErrUnexpectedEOF
		}
		cancel()
		return probe.verify(ctx, tool, env)
	}
	if mode == "verify error" {
		return deps, io.ErrUnexpectedEOF
	}
	return deps, context.Canceled
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestUninstallPreservesStdoutFailureRemedy(t *testing.T) {
	options, deps, probe := setupFixture(t)
	receiptPath, before := armedReceipt(t, options, deps)
	plan, err := setup.PlanUninstall(t.Context(), options, deps)
	if err != nil {
		t.Fatal(err)
	}
	const remedy = "restore access to the hooks directory"
	called := false
	deps.Run = func(ctx context.Context, command runner.Command) (runner.Output, error) {
		if len(command.Args) == 1 && command.Args[0] == "uninstall" {
			called = true
			return runner.Output{Code: 1, Stdout: []byte(remedy)}, nil
		}
		return probe.run(ctx, command)
	}
	if err := setup.Apply(t.Context(), plan, io.Discard, deps); err == nil || !strings.Contains(err.Error(), remedy) {
		t.Fatalf("uninstall remedy = %v", err)
	}
	if !called {
		t.Fatal("lefthook uninstall never ran")
	}
	requireUninstallRetained(t, probe, receiptPath, before, false)
}

func readUninstallEvidence(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func requireUninstallRetained(t *testing.T, probe *setupProbe, path string, receipt []byte, edited bool) {
	t.Helper()
	if after := readUninstallEvidence(t, path); !bytes.Equal(after, receipt) {
		t.Fatalf("receipt changed after refused uninstall: %q", after)
	}
	for _, name := range []string{"pre-commit", "commit-msg", "pre-push"} {
		want := fakeHook
		if edited && name == "pre-commit" {
			want = "edited after planning\n"
		}
		if data := readUninstallEvidence(t, filepath.Join(probe.hooks, name)); string(data) != want {
			t.Fatalf("hook %s = %q, want %q", name, data, want)
		}
	}
}

func requireNoUninstallCalls(t *testing.T, probe *setupProbe) {
	t.Helper()
	for _, command := range probe.calls {
		if len(command.Args) == 1 && command.Args[0] == "uninstall" {
			t.Fatalf("uninstall ran after refusal: %+v", command)
		}
	}
}

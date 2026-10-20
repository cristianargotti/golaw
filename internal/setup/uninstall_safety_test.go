package setup_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/setup"
)

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestUninstallAbsentReceiptNeedsNoExecutableOrAsset(t *testing.T) {
	options, deps, probe := setupFixture(t)
	for _, path := range []string{probe.golaw, filepath.Join(options.Root, "lefthook.yml")} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	runs, probes := 0, 0
	deps.Run = func(context.Context, runner.Command) (runner.Output, error) {
		runs++
		return runner.Output{}, io.ErrClosedPipe
	}
	deps.Verify = func(context.Context, runner.Tool, []string) (string, error) {
		probes++
		return "", runner.ErrToolMissing
	}
	plan, err := setup.PlanUninstall(t.Context(), options, deps)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := setup.Apply(t.Context(), plan, &out, deps); err != nil {
		t.Fatal(err)
	}
	if runs != 0 || probes != 0 || out.Len() != 0 {
		t.Fatalf("empty uninstall: runs=%d, probes=%d, output=%q", runs, probes, out.String())
	}
	if _, err := setup.ReadReceipt(plan.ReceiptPath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("empty uninstall created receipt: %v", err)
	}
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestUninstallNeedsOnlyRecordedOwnershipAndLefthook(t *testing.T) {
	for _, changed := range []string{"all hooks missing", "golaw missing", "asset missing", "mode only"} {
		t.Run(changed, func(t *testing.T) {
			options, deps, probe := setupFixture(t)
			receiptPath, _ := armedReceipt(t, options, deps)
			changeUninstallInputs(t, options, probe, changed)
			verify := deps.Verify
			deps.Verify = func(ctx context.Context, tool runner.Tool, env []string) (string, error) {
				if tool.Name != "lefthook" {
					t.Errorf("uninstall verified unrelated tool %s", tool.Name)
				}
				return verify(ctx, tool, env)
			}
			probe.calls = nil
			plan, err := setup.PlanUninstall(t.Context(), options, deps)
			if err != nil {
				t.Fatal(err)
			}
			if err := setup.Apply(t.Context(), plan, io.Discard, deps); err != nil {
				t.Fatal(err)
			}
			requirePlainUninstall(t, probe)
			requireUninstallGone(t, receiptPath, probe)
		})
	}
}

func changeUninstallInputs(t *testing.T, options setup.Options, probe *setupProbe, changed string) {
	t.Helper()
	var paths []string
	switch changed {
	case "all hooks missing":
		for _, name := range []string{"pre-commit", "commit-msg", "pre-push"} {
			paths = append(paths, filepath.Join(probe.hooks, name))
		}
	case "golaw missing":
		paths = append(paths, probe.golaw)
	case "asset missing":
		paths = append(paths, filepath.Join(options.Root, "lefthook.yml"))
	case "mode only":
		if err := os.Chmod(filepath.Join(probe.hooks, "pre-commit"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
}

func requirePlainUninstall(t *testing.T, probe *setupProbe) {
	t.Helper()
	uninstalls := 0
	for _, call := range probe.calls {
		name := strings.TrimSuffix(filepath.Base(call.Name), ".exe")
		if name == "git" {
			continue
		}
		if name != "lefthook" || !slices.Equal(call.Args, []string{"uninstall"}) {
			t.Fatalf("unexpected uninstall process: %s %v", call.Name, call.Args)
		}
		uninstalls++
	}
	if uninstalls != 1 {
		t.Fatalf("plain lefthook uninstall calls = %d", uninstalls)
	}
}

func requireUninstallGone(t *testing.T, receiptPath string, probe *setupProbe) {
	t.Helper()
	paths := make([]string, 0, 5)
	paths = append(paths, receiptPath, probe.checksum)
	for _, name := range []string{"pre-commit", "commit-msg", "pre-push"} {
		paths = append(paths, filepath.Join(probe.hooks, name))
	}
	for _, path := range paths {
		if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("uninstall kept %s: %v", path, err)
		}
	}
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestUninstallRefusesSymlinksEvenWithMatchingOwnedBytes(t *testing.T) {
	for _, kind := range []string{"hook", "directory"} {
		t.Run(kind, func(t *testing.T) {
			options, deps, probe := setupFixture(t)
			receiptPath, before := armedReceipt(t, options, deps)
			link, target := redirectUninstallHook(t, probe, kind)
			probe.calls = nil
			_, err := setup.PlanUninstall(t.Context(), options, deps)
			if !errors.Is(err, install.ErrChanged) {
				t.Fatalf("symlink ownership = %v", err)
			}
			requireUntouched(t, probe, receiptPath, before)
			if got, err := os.Readlink(link); err != nil || got != target {
				t.Fatalf("uninstall changed symlink: %q, %v", got, err)
			}
		})
	}
}

func redirectUninstallHook(t *testing.T, probe *setupProbe, kind string) (string, string) {
	t.Helper()
	link := filepath.Join(probe.hooks, "pre-commit")
	target := filepath.Join(t.TempDir(), "owned-hook")
	if kind == "directory" {
		link = probe.hooks
	}
	if err := os.Rename(link, target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	return link, target
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestUninstallRefusesReceiptForAnotherProject(t *testing.T) {
	for _, field := range []string{"identity", "root"} {
		t.Run(field, func(t *testing.T) {
			options, deps, probe := setupFixture(t)
			receiptPath, _ := armedReceipt(t, options, deps)
			before := changeUninstallReceipt(t, receiptPath, field)
			probe.calls = nil
			_, err := setup.PlanUninstall(t.Context(), options, deps)
			if !errors.Is(err, install.ErrChanged) {
				t.Fatalf("another project receipt = %v", err)
			}
			requireUntouched(t, probe, receiptPath, before)
		})
	}
}

func changeUninstallReceipt(t *testing.T, path, field string) []byte {
	t.Helper()
	receipt, err := setup.ReadReceipt(path)
	if err != nil {
		t.Fatal(err)
	}
	if field == "identity" {
		receipt.Identity += "-other"
	} else {
		receipt.Root = filepath.Join(receipt.Root, "other")
	}
	if writeErr := setup.WriteReceipt(path, receipt); writeErr != nil {
		t.Fatal(writeErr)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return before
}

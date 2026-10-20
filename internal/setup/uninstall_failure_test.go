package setup_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/cristianargotti/golaw/internal/setup"
)

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestUninstallStopsBeforeMutationWhenOutputOrContextFails(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cancel bool
	}{
		{"output failure", false},
		{"canceled application", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options, deps, probe := setupFixture(t)
			receiptPath, before := armedReceipt(t, options, deps)
			plan, err := setup.PlanUninstall(t.Context(), options, deps)
			if err != nil {
				t.Fatal(err)
			}
			failUninstall(t, plan, deps, probe, tc.cancel)
			requireUntouched(t, probe, receiptPath, before)
		})
	}
}

// armedReceipt arms the fixture with setup and answers the path and the
// bytes of the receipt it wrote.
func armedReceipt(t *testing.T, options setup.Options, deps setup.Dependencies) (string, []byte) {
	t.Helper()
	installed, err := setup.PlanInstall(t.Context(), options, deps)
	if err != nil {
		t.Fatal(err)
	}
	if applyErr := setup.Apply(t.Context(), installed, io.Discard, deps); applyErr != nil {
		t.Fatal(applyErr)
	}
	before, err := os.ReadFile(installed.ReceiptPath)
	if err != nil {
		t.Fatal(err)
	}
	return installed.ReceiptPath, before
}

// failUninstall applies an uninstall into a writer that fails, or under a
// canceled context, and requires that failure.
func failUninstall(t *testing.T, plan setup.Plan, deps setup.Dependencies, probe *setupProbe, canceled bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	var out io.Writer = setupBrokenWriter{}
	want := io.ErrClosedPipe
	if canceled {
		cancel()
		out = io.Discard
		want = context.Canceled
	}
	probe.calls = nil
	if err := setup.Apply(ctx, plan, out, deps); !errors.Is(err, want) {
		t.Fatalf("uninstall error = %v, want %v", err, want)
	}
}

// requireUntouched requires the receipt and the owned hooks unchanged and
// lefthook uninstall never started.
func requireUntouched(t *testing.T, probe *setupProbe, receiptPath string, before []byte) {
	t.Helper()
	after, err := os.ReadFile(receiptPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("receipt changed before uninstall: %v", err)
	}
	receipt, err := setup.ReadReceipt(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range receipt.Hooks {
		data, readErr := os.ReadFile(artifact.Path)
		if readErr != nil || string(data) != fakeHook {
			t.Fatalf("hook changed before uninstall: %q, %v", data, readErr)
		}
	}
	for _, call := range probe.calls {
		if len(call.Args) == 1 && call.Args[0] == "uninstall" {
			t.Fatalf("uninstall ran after failure: %+v", call)
		}
	}
}

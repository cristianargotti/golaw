package setup_test

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"testing"

	"github.com/cristianargotti/golaw/internal/setup"
)

type setupBrokenWriter struct{}

func (setupBrokenWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupIncompleteApplyCanRetry(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cancel bool
	}{
		{"output failure", false},
		{"canceled application", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options, deps, probe := setupFixture(t)
			plan, err := setup.PlanInstall(t.Context(), options, deps)
			if err != nil {
				t.Fatal(err)
			}
			failApply(t, plan, deps, probe, tc.cancel)
			requireRetry(t, options, deps)
		})
	}
}

// failApply applies setup into a writer that fails, or under a canceled
// context, and requires the failure, no receipt and, for a cancel, no
// installation.
func failApply(t *testing.T, plan setup.Plan, deps setup.Dependencies, probe *setupProbe, canceled bool) {
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
		t.Fatalf("application error = %v, want %v", err, want)
	}
	if _, err := setup.ReadReceipt(plan.ReceiptPath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("completed receipt after failure: %v", err)
	}
	if !canceled {
		return
	}
	for _, call := range probe.calls {
		if len(call.Args) != 0 && call.Args[0] == "install" {
			t.Fatalf("canceled application installed: %+v", call)
		}
	}
}

// requireRetry requires a setup after a failed one to complete with its
// three hooks.
func requireRetry(t *testing.T, options setup.Options, deps setup.Dependencies) {
	t.Helper()
	again, err := setup.PlanInstall(t.Context(), options, deps)
	if err != nil {
		t.Fatal(err)
	}
	if applyErr := setup.Apply(t.Context(), again, io.Discard, deps); applyErr != nil {
		t.Fatal(applyErr)
	}
	receipt, err := setup.ReadReceipt(again.ReceiptPath)
	if err != nil || len(receipt.Hooks) != 3 {
		t.Fatalf("retry receipt = %+v, %v", receipt, err)
	}
}

package doctor

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/setup"
)

func TestCollectSetupUntrustedReceiptsStartNoProcesses(t *testing.T) {
	for _, kind := range []string{"absent", "invalid", "foreign identity", "foreign root"} {
		t.Run(kind, func(t *testing.T) {
			f, want := untrustedCollectFixture(t, kind)
			runnertest.Install(t, f.bin, "git", runnertest.Behavior{Code: 2})
			t.Setenv("PATH", f.bin)
			facts, err := CollectSetup(t.Context(), f.root, f.state, gittest.Isolated(), model.DefaultLadder())
			if err != nil || facts.ReceiptState != want {
				t.Fatalf("receipt = %+v, %v", facts, err)
			}
			requireNoProbes(t, facts, f)
			if calls := runnertest.Calls(t, f.bin, "git"); len(calls) != 0 {
				t.Fatalf("untrusted receipt started Git: %v", calls)
			}
			if want == "invalid" && facts.ReceiptError == "" {
				t.Fatal("invalid receipt lost its reason")
			}
		})
	}
}

func untrustedCollectFixture(t *testing.T, kind string) (collectFixture, string) {
	t.Helper()
	receiptState := kind
	if strings.HasPrefix(kind, "foreign") {
		receiptState = "present"
	}
	f := newCollectFixture(t, collectCase{receipt: receiptState, version: "8.30.1"})
	if receiptState != "present" {
		return f, receiptState
	}
	if kind == "foreign identity" {
		f.receipt.Identity = "another-project"
	} else {
		f.receipt.Root = t.TempDir()
	}
	if err := setup.WriteReceipt(f.receiptPath, f.receipt); err != nil {
		t.Fatal(err)
	}
	return f, "invalid"
}

func TestCollectSetupRoutingFailuresKeepIndependentEvidence(t *testing.T) {
	for _, tc := range []struct {
		name     string
		behavior runnertest.Behavior
		want     string
	}{
		{"exit failure", runnertest.Behavior{Stderr: "configuration unreadable", Code: 2}, "configuration unreadable"},
		{"relative route", runnertest.Behavior{Stdout: "relative/hooks\n"}, "relative hooks directory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCollectFixture(t, collectCase{receipt: "present", version: "8.30.1"})
			runnertest.Install(t, f.bin, "git", tc.behavior)
			t.Setenv("PATH", f.bin)
			facts, err := CollectSetup(t.Context(), f.root, f.state, gittest.Isolated(), model.DefaultLadder())
			if err != nil || !strings.Contains(facts.HooksPathError, tc.want) {
				t.Fatalf("routing = %+v, %v", facts, err)
			}
			requireLiveEvidence(t, facts, f, collectCase{missing: true})
			for _, status := range facts.Tools {
				if status.Path == "" || status.Error != "" {
					t.Fatalf("Git failure suppressed tool evidence: %+v", status)
				}
			}
			requireSetupLine(t, facts, collectCase{id: "doctor.setup-routing", want: "unknown"})
		})
	}
}

func TestCollectSetupNamesAnUnavailableRoot(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "missing")
	_, err := CollectSetup(t.Context(), root, t.TempDir(), nil, model.DefaultLadder())
	if !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), "identify setup repository") {
		t.Fatalf("missing root = %v", err)
	}
}

func TestSetupHookEvidenceRejectsMissingAndNonregularPaths(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	missing := filepath.Join(root, "missing")
	directory := filepath.Join(root, "directory")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path, state string
	}{
		{missing, "missing"},
		{directory, "unknown"},
	} {
		hook := collectSetupHook(tc.path)
		if hook.Path != tc.path || hook.Error == "" || hook.SHA256 != "" {
			t.Fatalf("unreadable hook = %+v", hook)
		}
		facts := setupBaseline()
		facts.Receipt.Hooks[0].Path = tc.path
		facts.Hooks = []SetupHook{hook}
		requireSetupLine(t, facts, collectCase{id: "doctor.setup-hooks", want: tc.state})
	}
	regular := filepath.Join(root, "pre-commit")
	data := []byte("owned hook\n")
	if err := os.WriteFile(regular, data, 0o700); err != nil {
		t.Fatal(err)
	}
	hook := collectSetupHook(regular)
	if hook.Path != regular || hook.Error != "" || hook.SHA256 != install.Digest(data) {
		t.Fatalf("regular hook = %+v", hook)
	}
}

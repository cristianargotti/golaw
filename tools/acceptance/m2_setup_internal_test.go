package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/setup"
)

func TestM2SetupVerifiedEnvironment(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t).m2Scope("setup-verified")
	s.execute = m2LifecycleScript(t, s, "setup-idempotent", "answer").Execute
	repo, err := s.repository(t.Context(), "setup-verified", true)
	if err != nil {
		t.Fatal(err)
	}
	env, err := s.setupM2(t.Context(), repo, s.env)
	if err != nil {
		t.Fatal(err)
	}
	if environmentValue(env, "GOBIN") != s.m2.Bin {
		t.Fatalf("shared tool directory missing from returned environment: %q", env)
	}
}

// Git for Windows prints the hooks directory with forward slashes while
// setup records the cleaned native path; the trailing slash gives the
// report a spelling filepath.Clean folds on every system.
func TestM2SetupComparesTheHooksDirectoryAsSetupRecordsIt(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t).m2Scope("setup-spelling")
	fixture := m2LifecycleScript(t, s, "setup-idempotent", "answer").Execute
	s.execute = func(ctx context.Context, c command) result {
		got := fixture(ctx, c)
		if c.program == s.git && slices.Contains(c.args, "--git-path") && slices.Contains(c.args, "hooks") {
			got.stdout = filepath.ToSlash(strings.TrimSpace(got.stdout)) + "/\n"
		}
		return got
	}
	repo, err := s.repository(t.Context(), "setup-spelling", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.setupM2(t.Context(), repo, s.env); err != nil {
		t.Fatalf("the hooks directory Git spells another way was refused: %v", err)
	}
	if _, err := s.m2InstalledPath(t.Context(), repo, "pre-commit", s.env); err != nil {
		t.Fatalf("the installed rung under that spelling was refused: %v", err)
	}
}

func TestM2SetupRejectsIncompleteEvidence(t *testing.T) {
	t.Parallel()
	modes := []string{
		"missing-receipt", "wrong-identity", "wrong-version", "wrong-candidate", "missing-candidate",
		"missing-tool", "tool-pin", "tool-count", "hook-hash", "hook-count", "hook-route",
		"probe-error", "doctor-error", "doctor-invalid", "doctor-schema", "doctor-receipt",
		"doctor-hooks", "doctor-tool", "doctor-split", "doctor-failed", "doctor-empty",
	}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			s := m2TestSuite(t).m2Scope(mode)
			script := m2LifecycleScript(t, s, "setup-idempotent", "answer")
			s.execute = m2SetupFaultExecutor(t, s, script.Execute, mode)
			repo, err := s.repository(t.Context(), mode, true)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.setupM2(t.Context(), repo, s.env); err == nil {
				t.Fatalf("accepted %s", mode)
			}
		})
	}
}

func m2SetupFaultExecutor(t *testing.T, s *suite, fixture executor, mode string) executor {
	t.Helper()
	return func(ctx context.Context, c command) result {
		got := fixture(ctx, c)
		switch m2Kind(s, c) {
		case m2SetupCommand:
			m2AlterSetup(t, s, c, mode)
		case "probe":
			if mode == "probe-error" {
				return result{err: ErrAcceptance}
			}
		case "doctor":
			return m2AlterDoctor(t, got, mode)
		}
		return got
	}
}

func m2AlterSetup(t *testing.T, s *suite, c command, mode string) {
	t.Helper()
	path := m2ReceiptPath(t, s, c)
	receipt, err := setup.ReadReceipt(path)
	if err != nil {
		t.Fatal(err)
	}
	switch mode {
	case "missing-receipt":
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		return
	case "wrong-identity":
		receipt.Identity = "another"
	case "wrong-version":
		receipt.Version = "0.1.0"
	case "wrong-candidate":
		receipt.Golaw = filepath.Join(s.base, "other-candidate")
		m2Write(t, receipt.Golaw, m2Executable)
	case "missing-candidate":
		receipt.Golaw = filepath.Join(s.base, "missing-candidate")
	case "missing-tool":
		if err := os.Remove(receipt.Tools[0].Path); err != nil {
			t.Fatal(err)
		}
	case "tool-pin":
		receipt.Tools[0].Version = "0.0.1"
	case "tool-count":
		receipt.Tools = receipt.Tools[1:]
	case "hook-hash":
		m2Write(t, receipt.Hooks[0].Path, "changed hook")
	case "hook-count":
		receipt.Hooks = receipt.Hooks[1:]
	case "hook-route":
		receipt.HooksPath = filepath.Join(s.base, "other-hooks")
		for i := range receipt.Hooks {
			receipt.Hooks[i].Path = filepath.Join(receipt.HooksPath, filepath.Base(receipt.Hooks[i].Path))
		}
	}
	if err := setup.WriteReceipt(path, receipt); err != nil {
		t.Fatal(err)
	}
}

func m2AlterDoctor(t *testing.T, got result, mode string) result {
	t.Helper()
	switch mode {
	case "doctor-error":
		return result{err: ErrAcceptance}
	case "doctor-invalid":
		return result{stdout: "{"}
	case "doctor-empty":
		return result{}
	}
	// The setup lines live in the first document, the Claude report; the
	// Codex and Cursor documents follow it unchanged.
	reports, err := decodeReports(got.stdout)
	if err != nil {
		t.Fatal(err)
	}
	doc := &reports[0]
	switch mode {
	case "doctor-schema":
		doc.Schema++
	case "doctor-receipt":
		doc.Lines[0].Detail = "absent"
	case "doctor-hooks":
		doc.Lines[1].Detail = "absent"
	case "doctor-tool":
		doc.Lines = doc.Lines[:len(doc.Lines)-1]
	case "doctor-split":
		tool := runner.Toolchain()[0]
		doc.Lines[2].Detail = strings.ReplaceAll(doc.Lines[2].Detail, tool.Version, "unknown")
		doc.Lines = append(doc.Lines, doctor.Line{State: "ok", Detail: tool.Version})
	case "doctor-failed":
		doc.Lines[2].State = "fail"
	}
	documents := make([]string, 0, len(reports))
	for _, report := range reports {
		data, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		documents = append(documents, string(data)+"\n")
	}
	got.stdout = strings.Join(documents, "")
	return got
}

func TestM2CleanSetupFailureRemainsUnready(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t).m2Scope("clean-failure")
	script := m2LifecycleScript(t, s, "setup-clean", "corrupt")
	s.execute = script.Execute
	repo, err := s.repository(t.Context(), "clean-failure", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.m2SetupClean(t.Context(), repo); err == nil || s.m2.Ready {
		t.Fatalf("clean failure ready=%v error=%v", s.m2.Ready, err)
	}
	before := script.Calls
	if _, err := s.setupM2(t.Context(), repo, s.env); err == nil || script.Calls != before {
		t.Fatalf("unverified tools reused: error=%v calls=%d before=%d", err, script.Calls, before)
	}
}

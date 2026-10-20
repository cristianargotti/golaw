package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

func TestM2SetupRejectsIncompleteEvidence(t *testing.T) {
	t.Parallel()
	modes := []string{
		"missing-receipt", "wrong-identity", "wrong-version", "wrong-candidate", "missing-candidate",
		"missing-tool", "tool-pin", "tool-count", "hook-hash", "hook-count", "hook-route",
		"probe-error", "doctor-error", "doctor-invalid", "doctor-schema", "doctor-receipt",
		"doctor-hooks", "doctor-tool", "doctor-split", "doctor-failed",
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
	}
	var doc doctor.Report
	if err := json.Unmarshal([]byte(got.stdout), &doc); err != nil {
		t.Fatal(err)
	}
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
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	got.stdout = string(data)
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

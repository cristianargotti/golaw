package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/setup"
)

func TestM2SetupPathFailures(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	s.m2Prerequisites = nil
	if _, err := s.m2StateDir(nil); err == nil {
		t.Fatal("missing state environment accepted")
	}
	if _, err := s.m2ReceiptAt(t.TempDir(), nil); err == nil {
		t.Fatal("missing receipt environment accepted")
	}
	if _, err := s.m2ReceiptAt(filepath.Join(t.TempDir(), "missing"), s.env); err == nil {
		t.Fatal("missing repository identity accepted")
	}
	s.execute = func(context.Context, command) result { return result{stdout: "relative-hooks"} }
	if _, err := s.m2EffectiveHooks(t.Context(), t.TempDir(), s.env); err == nil {
		t.Fatal("relative hooks routing accepted")
	}
	s.execute = func(context.Context, command) result { return result{err: ErrAcceptance} }
	if _, err := s.m2EffectiveHooks(t.Context(), t.TempDir(), s.env); err == nil {
		t.Fatal("failed hooks routing accepted")
	}
	s.base = filepath.Join(t.TempDir(), "file")
	m2Write(t, s.base, "occupied")
	_, err := s.m2PrerequisiteEnvironment(s.env, s.m2.Bin)
	if (err != nil) != (s.m2OS != goosWindows) {
		t.Fatalf("prerequisite directory handling differs for %s: %v", s.m2OS, err)
	}
}

func TestM2SetupPrerequisiteIsolation(t *testing.T) {
	t.Parallel()
	s, _ := m2PrerequisiteSuite(t, runtime.GOOS)
	foreign := t.TempDir()
	m2Write(t, filepath.Join(foreign, executable("gitleaks")), "foreign tool")
	env, err := s.m2PrerequisiteEnvironment(append(s.env, pathAssignment+foreign), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := m2MissingTools(env); err != nil {
		t.Fatal(err)
	}
	if _, err := s.m2PrerequisiteEnvironment(env, t.TempDir()); err != nil {
		t.Fatalf("prerequisites are not reusable: %v", err)
	}
	if err := m2MissingTools([]string{pathAssignment + foreign}); err == nil {
		t.Fatal("resolving optional tool accepted as absent")
	}
	occupied := filepath.Join(t.TempDir(), "occupied")
	m2Write(t, occupied, "occupied")
	if err := m2MissingTools([]string{pathAssignment + occupied}); err == nil {
		t.Fatal("unreadable tool search accepted")
	}
}

func TestM2PrerequisiteLinkFailures(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	path := filepath.Join(base, "link")
	if err := m2PrerequisiteLink("source", path); err != nil {
		if runtime.GOOS == goosWindows {
			t.Skip("Windows setup uses executable directories without requiring symlink permission")
		}
		t.Fatal(err)
	}
	if err := m2PrerequisiteLink("changed", path); err == nil {
		t.Fatal("changed prerequisite target accepted")
	}
	occupied := filepath.Join(base, "occupied")
	m2Write(t, occupied, "occupied")
	if err := m2PrerequisiteLink("source", occupied); err == nil {
		t.Fatal("regular file accepted as prerequisite link")
	}
	if err := m2PrerequisiteLink("source", filepath.Join(occupied, "link")); err == nil {
		t.Fatal("invalid prerequisite parent accepted")
	}
	if err := m2PrerequisiteLink("source", filepath.Join(base, "missing", "link")); err == nil {
		t.Fatal("link creation failure accepted")
	}
}

func TestM2SetupExecutableFailures(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	plain := filepath.Join(root, "plain")
	if err := writeFile(plain, []byte("not executable"), fileMode); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{root, filepath.Join(root, "missing")} {
		if err := m2RequireExecutable(path); err == nil {
			t.Fatalf("nonexecutable path accepted: %s", path)
		}
	}
	if os.PathSeparator != '\\' {
		if err := m2RequireExecutable(plain); err == nil {
			t.Fatal("missing executable bits accepted")
		}
	}
}

func TestM2SetupUninstallEvidenceFailures(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"receipt-retained", "hook-retained", "local-missing", "local-changed", "tool-changed", "tools-missing"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			s := m2TestSuite(t)
			root := t.TempDir()
			path, local := filepath.Join(root, "receipt"), filepath.Join(root, "post-merge")
			receipt := setup.Receipt{Hooks: nil}
			m2Write(t, local, m2LocalEdit)
			before, err := treeDigests(s.m2.Bin)
			if err != nil {
				t.Fatal(err)
			}
			m2UninstallFault(t, s, mode, path, local, &receipt)
			if err := s.m2VerifyUninstall(path, local, receipt, before); err == nil {
				t.Fatalf("uninstall accepted %s", mode)
			}
		})
	}
}

func m2UninstallFault(t *testing.T, s *suite, mode, path, local string, receipt *setup.Receipt) {
	t.Helper()
	switch mode {
	case "receipt-retained":
		m2Write(t, path, "retained")
	case "hook-retained":
		receipt.Hooks = append(receipt.Hooks, install.Artifact{Path: local})
	case "local-missing":
		if err := os.Remove(local); err != nil {
			t.Fatal(err)
		}
	case "local-changed":
		m2Write(t, local, "changed")
	case "tool-changed":
		m2Write(t, filepath.Join(s.m2.Bin, executable("lefthook")), "changed")
	case "tools-missing":
		if err := os.RemoveAll(s.m2.Bin); err != nil {
			t.Fatal(err)
		}
	}
}

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
)

func TestM2InterruptionKeepsWarmCachesAndPrivateTools(t *testing.T) {
	t.Parallel()
	for _, live := range []bool{false, true} {
		s := m2TestSuite(t)
		s.m2.Live = live
		original := slices.Clone(s.env)
		script := m2LifecycleScript(t, s, "setup-interrupted", "answer")
		builds := 0
		s.execute = func(ctx context.Context, c command) result {
			if c.program == s.goTool {
				builds++
				if len(c.args) == 0 || c.args[0] != goBuild {
					t.Fatalf("unexpected Go operation: %+v", c)
				}
			}
			return script.Execute(ctx, c)
		}
		probe, err := s.m2PrepareInterruption(t.Context(), t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		bin := filepath.Join(s.base, "interrupted-tools")
		if probe.program != filepath.Join(bin, executable("go")) || builds != 1 ||
			!slices.Equal(s.env, original) || script.Targets != 0 {
			t.Fatalf("probe=%+v builds=%d targets=%d", probe, builds, script.Targets)
		}
		m2RequireInterruptionEnvironment(t, s, probe, original, bin)
		m2RequireInterruptedCopies(t, s, bin)
	}
}

func m2RequireInterruptionEnvironment(t *testing.T, s *suite, probe m2Interruption, original []string, bin string) {
	t.Helper()
	for _, key := range []string{"HOME", "GOCACHE", "GOMODCACHE"} {
		if environmentValue(probe.env, key) != environmentValue(original, key) {
			t.Fatalf("%s lost its case home or shared cache", key)
		}
	}
	for key, want := range map[string]string{
		"GOBIN": bin, "GOLAW_M2_REAL_GO": s.goTool,
		"GOLAW_M2_BINARY": s.binary, "GOLAW_M2_MARKER": probe.marker,
	} {
		if got := environmentValue(probe.env, key); got != want {
			t.Fatalf("%s=%q want %q", key, got, want)
		}
	}
	if !strings.HasPrefix(environmentValue(probe.env, envPath), bin+string(os.PathListSeparator)) {
		t.Fatal("controller directory does not take precedence")
	}
}

func m2RequireInterruptedCopies(t *testing.T, s *suite, bin string) {
	t.Helper()
	tools := runner.Toolchain()
	for _, tool := range tools[:len(tools)-1] {
		source, err := os.ReadFile(filepath.Join(s.m2.Bin, executable(tool.Name)))
		if err != nil {
			t.Fatal(err)
		}
		copied, err := os.ReadFile(filepath.Join(bin, executable(tool.Name)))
		if err != nil || !bytes.Equal(source, copied) {
			t.Fatalf("%s was not copied: %v", tool.Name, err)
		}
	}
	if err := m2InterruptAbsent(filepath.Join(bin, executable(tools[len(tools)-1].Name))); err != nil {
		t.Fatal(err)
	}
}

func TestM2InterruptedSetupRestoresNormalExecution(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	script := m2LifecycleScript(t, s, "setup-interrupted", "answer")
	setups, controllers := 0, 0
	s.execute = func(ctx context.Context, c command) result {
		switch m2Kind(s, c) {
		case "controller":
			controllers++
			m2RequireInterruptedCopies(t, s, environmentValue(c.env, "GOBIN"))
		case "setup":
			setups++
			if controllers != 1 || environmentValue(c.env, "GOBIN") != s.m2.Bin ||
				environmentValue(c.env, "GOLAW_M2_REAL_GO") != "" ||
				strings.Contains(environmentValue(c.env, envPath), "interrupted-tools") {
				t.Fatalf("retry kept interrupted preparation: %+v", c)
			}
		}
		return script.Execute(ctx, c)
	}
	if err := s.lifecycleM2(t.Context(), "setup-interrupted"); err != nil {
		t.Fatal(err)
	}
	if controllers != 1 || setups != 2 {
		t.Fatalf("controller=%d setup=%d", controllers, setups)
	}
}

func TestM2InterruptedEvidenceRequiresCompletionAndNoReceipt(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"valid", "execution", "exit", "output", "marker", "release", "done", "empty", "receipt"} {
		base := t.TempDir()
		marker, receipt := filepath.Join(base, "marker"), filepath.Join(base, "receipt")
		for _, path := range []string{marker, marker + ".release", marker + ".done"} {
			m2Write(t, path, "completed\n")
		}
		got := result{stdout: "installation output\n" + m2InterruptedMarker + "\r\n"}
		switch mode {
		case "execution":
			got.err = ErrAcceptance
		case "exit":
			got.code = 1
		case "output":
			got.stdout = "not " + m2InterruptedMarker + "\n"
		case "marker", "release", "done":
			path := marker
			if mode != "marker" {
				path += "." + mode
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		case "empty":
			m2Write(t, marker+".done", "")
		case "receipt":
			m2Write(t, receipt, "premature\n")
		}
		if err := m2InterruptedEvidence(got, marker, receipt); (err != nil) != (mode != "valid") {
			t.Fatalf("%s: evidence error=%v", mode, err)
		}
	}
}

func TestM2InterruptedSetupRejectsStaleEvidenceAndUnverifiedTools(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"unverified", "marker", "release", "done"} {
		s := m2TestSuite(t)
		script := m2LifecycleScript(t, s, "setup-interrupted", "answer")
		s.execute = script.Execute
		if mode == "unverified" {
			s.m2.Ready = false
		} else {
			marker := filepath.Join(s.base, "installation-marker")
			if mode != "marker" {
				marker += "." + mode
			}
			m2Write(t, marker, "stale\n")
		}
		if err := s.lifecycleM2(t.Context(), "setup-interrupted"); !errors.Is(err, ErrAcceptance) ||
			script.Targets != 0 {
			t.Fatalf("%s: error=%v targets=%d", mode, err, script.Targets)
		}
	}
}

func TestM2InterruptionPreparationFailures(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{
		"missing-tool", "wrong-pin", "probe-error", "build-error",
		"missing-fixture", "blocked-source", "missing-tool-present", "blocked-bin",
	} {
		s := m2TestSuite(t)
		script := m2LifecycleScript(t, s, "setup-interrupted", "answer")
		s.execute = func(ctx context.Context, c command) result {
			if m2Kind(s, c) == "probe" {
				switch mode {
				case "wrong-pin":
					return result{stdout: "another pin\n"}
				case "probe-error":
					return result{err: ErrAcceptance}
				}
			}
			if mode == "build-error" && c.program == s.goTool {
				return result{code: 1}
			}
			return script.Execute(ctx, c)
		}
		m2InterruptFaultInputs(t, s, mode)
		if _, err := s.m2PrepareInterruption(t.Context(), t.TempDir()); err == nil || script.Targets != 0 {
			t.Fatalf("%s: error=%v targets=%d", mode, err, script.Targets)
		}
	}
}

func m2InterruptFaultInputs(t *testing.T, s *suite, mode string) {
	t.Helper()
	switch mode {
	case "missing-tool":
		tool := runner.Toolchain()[0]
		if err := os.Remove(filepath.Join(s.m2.Bin, executable(tool.Name))); err != nil {
			t.Fatal(err)
		}
	case "missing-fixture":
		s.root = t.TempDir()
	case "blocked-source":
		m2Write(t, filepath.Join(s.base, "interrupt-probe"), "occupied\n")
	case "missing-tool-present":
		tools := runner.Toolchain()
		m2Write(t, filepath.Join(s.base, "interrupted-tools", executable(tools[len(tools)-1].Name)), "present\n")
	case "blocked-bin":
		m2Write(t, filepath.Join(s.base, "interrupted-tools"), "occupied\n")
	}
}

func TestM2InterruptionRefusesUnreadableAbsence(t *testing.T) {
	t.Parallel()
	if err := m2InterruptAbsent(filepath.Join(m2UnreadableParent(t), "receipt")); err == nil {
		t.Fatal("failed absence inspection passed")
	}
}

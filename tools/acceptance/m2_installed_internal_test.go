package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/setup"
)

func TestM2InstalledInvocationPreservesItsInputs(t *testing.T) {
	t.Parallel()
	s, repo := m2PreparedNative(t)
	args := []string{"local remote", "path with space"}
	env := append(slices.Clone(s.env), "M2_EXACT_INPUT=kept")
	invocation, err := s.installedM2(t.Context(), repo, "pre-push", "ref stream\n", args, env)
	if err != nil {
		t.Fatal(err)
	}
	shell, err := s.lookup("sh")
	if err != nil {
		t.Fatal(err)
	}
	if invocation.program != shell || invocation.dir != repo || invocation.input != "ref stream\n" ||
		!slices.Equal(invocation.args[1:], args) || !slices.Equal(invocation.env, env) {
		t.Fatalf("installed invocation lost its inputs: %+v", invocation)
	}
	hooks, err := s.m2EffectiveHooks(t.Context(), repo, env)
	if err != nil || invocation.args[0] != filepath.Join(hooks, "pre-push") {
		t.Fatalf("installed invocation did not use routed hook: %+v %v", invocation, err)
	}
	args[0], env[0] = "changed", "changed"
	if invocation.args[1] != "local remote" || invocation.env[0] == "changed" {
		t.Fatal("installed invocation aliases mutable inputs")
	}
}

func m2PreparedNative(t *testing.T) (*suite, string) {
	t.Helper()
	s := m2TestSuite(t).m2Scope("installed-invocation")
	s.execute = m2NativeScript(t, s, "installed-pre-commit-near", "answer").Execute
	repo, err := s.m2NativeRepository(t.Context(), "installed-invocation")
	if err != nil {
		t.Fatal(err)
	}
	env, err := s.setupM2(t.Context(), repo, s.env)
	if err != nil {
		t.Fatal(err)
	}
	s.env = env
	return s, repo
}

func TestM2InstalledInvocationRequiresOwnedRoutedHooks(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"unknown", "missing-receipt", "identity", "routing", "hook", "shell", "shell-file", "git"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			s, repo := m2PreparedNative(t)
			rungName := "pre-commit"
			if mode == "unknown" {
				rungName = mode
			}
			m2BreakInstalled(t, s, repo, mode)
			if _, err := s.installedM2(t.Context(), repo, rungName, "", nil, s.env); err == nil {
				t.Fatalf("accepted %s installed invocation", mode)
			}
		})
	}
}

func m2BreakInstalled(t *testing.T, s *suite, repo, mode string) {
	t.Helper()
	path, err := s.m2ReceiptAt(repo, s.env)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := setup.ReadReceipt(path)
	if err != nil {
		t.Fatal(err)
	}
	switch mode {
	case "missing-receipt":
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	case "identity", "routing":
		if mode == "identity" {
			receipt.Version = m2OlderVersion
		} else {
			receipt.HooksPath = filepath.Join(s.base, "other-hooks")
			for i := range receipt.Hooks {
				receipt.Hooks[i].Path = filepath.Join(receipt.HooksPath, filepath.Base(receipt.Hooks[i].Path))
			}
		}
		if err := setup.WriteReceipt(path, receipt); err != nil {
			t.Fatal(err)
		}
	case "hook":
		m2Write(t, receipt.Hooks[0].Path, "changed hook")
	case "shell":
		s.lookup = func(string) (string, error) { return "", ErrAcceptance }
	case "shell-file":
		s.lookup = func(string) (string, error) { return t.TempDir(), nil }
	case "git":
		s.execute = func(context.Context, command) result { return result{err: ErrAcceptance} }
	}
}

func TestM2NativePrecommitRejectsStateChanges(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"checkout", "index", "missing-index"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			s := m2TestSuite(t)
			script := m2NativeScript(t, s, "installed-pre-commit-near", "answer")
			s.execute = func(ctx context.Context, c command) result {
				got := script.Execute(ctx, c)
				if m2InstalledHook(s, c) {
					m2MutateInstalled(t, c.dir, mode)
				}
				return got
			}
			if got := s.nativeM2(t.Context(), "installed-pre-commit-near"); got.status != statusFail || script.Targets != 1 {
				t.Fatalf("%s: target=%d result=%+v", mode, script.Targets, got)
			}
		})
	}
}

func m2MutateInstalled(t *testing.T, repo, mode string) {
	t.Helper()
	switch mode {
	case "checkout":
		m2Write(t, filepath.Join(repo, pathNote), "changed by hook")
	case "index":
		m2Write(t, filepath.Join(repo, gitDirectory, "index"), "invalid index")
	case "missing-index":
		if err := os.Remove(filepath.Join(repo, gitDirectory, "index")); err != nil {
			t.Fatal(err)
		}
	}
}

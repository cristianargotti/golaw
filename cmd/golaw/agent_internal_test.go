package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
)

// fakePort records the enrollments the installer writes.
type fakePort struct {
	calls  int
	digest string
}

func (p *fakePort) WriteEnrollment(_ context.Context, _, policyDigest, profileDigest string) (model.Enrollment, error) {
	p.calls, p.digest = p.calls+1, policyDigest
	return model.Enrollment{State: model.EnrollmentEnrolled, PolicyDigest: policyDigest, ProfileDigest: profileDigest}, nil
}

func (p *fakePort) ReadEnrollment(context.Context, string, string, string) (model.Enrollment, error) {
	return model.Enrollment{State: model.EnrollmentEnrolled, PolicyDigest: p.digest}, nil
}

// agentApp wires the agent commands with fakes: the installer writes into a
// temporary home, the port records the enrollment, the repository is a
// temporary directory.
func agentApp(t *testing.T) (testApp, *fakePort, string) {
	t.Helper()
	ta := newTestApp(t)
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(home, "repository")
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(home, "state")
	port := &fakePort{}
	ta.installOptions = func(_ context.Context, _ agents.Agent, root, scope string, _ time.Time) (install.Options, error) {
		return install.Options{
			Root: root, ConfigDir: filepath.Join(home, "config"), StateDir: state, Scope: scope,
			Version: "9.9.9", PolicyDigest: "sha256:policy", ReadRules: []string{"Read(**/*.rule)"},
		}, nil
	}
	ta.receiptPath = func(agent agents.Agent, root, scope string) (string, error) {
		_, identity, err := install.ProjectIdentity(root)
		if err != nil {
			return "", fmt.Errorf("identify %s: %w", root, err)
		}
		return agents.ReceiptPath(state, agent.ID, scope, identity), nil
	}
	ta.enrollments = func(string, time.Time) (enrollmentPort, error) { return port, nil }
	return ta, port, root
}

func TestAgentInstallTwiceThenUninstall(t *testing.T) {
	t.Parallel()
	ta, port, root := agentApp(t)
	id := agents.IDs()[0]
	if got := ta.run(t.Context(), []string{"agent", "install", "--agent", id, root}); got != exitClean {
		t.Fatalf("install exit = %d, stderr = %q", got, ta.stderr.String())
	}
	first := ta.stdout.String()
	if !strings.Contains(first, "installed: ") || !strings.Contains(first, "+++ ") || port.calls != 1 || port.digest != "sha256:policy" {
		t.Fatalf("first install = %q, port = %+v", first, port)
	}
	receipt := strings.TrimSpace(strings.TrimPrefix(first[strings.LastIndex(first, "installed: "):], "installed: "))
	if _, err := os.Stat(receipt); err != nil {
		t.Fatal(err)
	}
	ta.stdout.Reset()
	if got := ta.run(t.Context(), []string{"agent", "install", "--agent", id, root}); got != exitClean {
		t.Fatalf("second install exit = %d, stderr = %q", got, ta.stderr.String())
	}
	if second := ta.stdout.String(); strings.Contains(second, "+++ ") || strings.Contains(second, "--- ") || port.calls != 2 {
		t.Fatalf("second install printed a diff: %q", second)
	}
	ta.stdout.Reset()
	if got := ta.run(t.Context(), []string{"agent", "uninstall", "--agent", id, root}); got != exitClean {
		t.Fatalf("uninstall exit = %d, stderr = %q", got, ta.stderr.String())
	}
	if _, err := os.Stat(receipt); err == nil || !strings.Contains(ta.stdout.String(), "uninstalled: ") {
		t.Fatalf("the receipt survived: %q", ta.stdout.String())
	}
}

func TestAgentRefusesWhileTheGateOwnsAFamily(t *testing.T) {
	t.Parallel()
	ta, port, root := agentApp(t)
	h := gateHome(t, root)
	ta.home = func() (string, error) { return h, nil }
	if got := ta.run(t.Context(), []string{"agent", "install", "--agent", agents.IDs()[0], root}); got != exitFindings {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	if !strings.Contains(ta.stdout.String(), "refused: the Python gate still owns a blocking verdict") || port.calls != 0 {
		t.Fatalf("stdout = %q, port = %+v", ta.stdout.String(), port)
	}
}

func TestAgentReportsUsageErrorsAndSeamFailures(t *testing.T) {
	t.Parallel()
	id := agents.IDs()[0]
	cases := map[string]struct {
		args   []string
		stderr string
	}{
		"no agent":      {[]string{"agent", "install"}, "--agent is required"},
		"unknown agent": {[]string{"agent", "install", "--agent", "nobody"}, `unknown agent "nobody"`},
		"bad scope":     {[]string{"agent", "install", "--agent", id, "--scope", "team"}, "--scope must be user or project"},
		"uninstall":     {[]string{"agent", "uninstall", "--agent", "nobody"}, "agent uninstall: "},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ta, _, root := agentApp(t)
			if got := ta.run(t.Context(), append(tc.args, root)); got != exitFailure || !strings.Contains(ta.stderr.String(), tc.stderr) {
				t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
			}
		})
	}
	broken, _, root := agentApp(t)
	broken.discover = func(context.Context, string) (string, error) { return "", errBoom }
	if got := broken.run(t.Context(), []string{"agent", "install", "--agent", id, root}); got != exitFailure || broken.stderr.String() != "golaw: agent install: boom\n" {
		t.Fatalf("exit = %d, stderr = %q", got, broken.stderr.String())
	}
	missing, _, root := agentApp(t)
	if got := missing.run(t.Context(), []string{"agent", "uninstall", "--agent", id, root}); got != exitFailure || !strings.Contains(missing.stderr.String(), "read receipt") {
		t.Fatalf("exit = %d, stderr = %q", got, missing.stderr.String())
	}
	options, _, root := agentApp(t)
	options.installOptions = func(context.Context, agents.Agent, string, string, time.Time) (install.Options, error) {
		return install.Options{}, errBoom
	}
	if got := options.run(t.Context(), []string{"agent", "install", "--agent", id, root}); got != exitFailure || options.stderr.String() != "golaw: agent install: boom\n" {
		t.Fatalf("exit = %d, stderr = %q", got, options.stderr.String())
	}
}

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// freshMachineApp is the app of a machine without the bundle: the production
// binding over a temporary home, the catalog, the scan, the init and the
// agent install seams, and nothing installed under the config directory.
func freshMachineApp(t *testing.T) testApp {
	t.Helper()
	ta := seedApp(t)
	b := ta.bind()
	ta.installOptions, ta.receiptPath, ta.enrollments = b.installOptions, b.receiptPath, b.enrollments
	return ta
}

// oneInstallLine requires the output of a governed operation to name the
// install command of the fixture on exactly one line.
func oneInstallLine(t *testing.T, name, out string) {
	t.Helper()
	named := 0
	for line := range strings.SplitSeq(strings.TrimSuffix(out, "\n"), "\n") {
		if strings.Contains(line, "golaw profile install") {
			named++
			if !strings.Contains(line, fixtureInstallCommand) {
				t.Errorf("%s names another command: %q", name, line)
			}
		}
	}
	if named != 1 {
		t.Errorf("%s: %d lines name the install command:\n%s", name, named, out)
	}
}

// readTestFile reads a file a test wrote.
func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestFreshMachineGetsOneErrorOnEveryGovernedOperation(t *testing.T) {
	t.Parallel()
	root := profiledRepository(t)
	enrolled := digestOf(readTestFile(t, filepath.Join(root, "golaw.yml"))) + "," + profiletest.Ref(t).Digest
	for name, args := range map[string][]string{
		"scan":          {"scan", root},
		"scan staged":   {"scan", "--staged", root},
		"scan enrolled": {"scan", "--enrolled", enrolled, root},
		"init apply":    {"init", "--apply", root},
		"agent install": {"agent", "install", "--agent", agents.IDs()[0], root},
		"setup":         {"setup", root},
	} {
		ta := freshMachineApp(t)
		if code := ta.run(t.Context(), args); code != exitFailure || ta.stdout.Len() != 0 {
			t.Errorf("%s: exit %d, stdout %q", name, code, ta.stdout.String())
		}
		if lines := strings.Count(ta.stderr.String(), "\n"); lines != 1 {
			t.Errorf("%s: %d stderr lines: %q", name, lines, ta.stderr.String())
		}
		oneInstallLine(t, name, ta.stderr.String())
	}
}

//nolint:paralleltest // the rung runs in the directory of the repository, which t.Chdir sets
func TestFreshMachineArmedRungRefusesWithTheInstallCommand(t *testing.T) {
	root := profiledRepository(t)
	ta := freshMachineApp(t)
	armed(t, ta, root)
	t.Chdir(root)
	if code := ta.run(t.Context(), []string{"hook", "pre-commit"}); code != exitFailure {
		t.Fatalf("exit %d, stdout %q", code, ta.stdout.String())
	}
	oneInstallLine(t, "pre-commit", ta.stdout.String())
	off := freshMachineApp(t)
	armed(t, off, root)
	policyPath := filepath.Join(root, "golaw.yml")
	writeFiles(t, root, map[string]string{"golaw.yml": strings.Replace(string(readTestFile(t, policyPath)), "ladder: { pre_commit: true", "ladder: { pre_commit: false", 1)})
	if code := off.run(t.Context(), []string{"hook", "pre-commit"}); code != exitClean || off.stdout.Len() != 0 {
		t.Errorf("a rung switched off: exit %d, stdout %q", code, off.stdout.String())
	}
}

// freshHook answers the reply of the first agent's hook to a write in a
// repository whose golaw.yml pins the fixture, on a machine without the
// bundle, with the repository enrolled on that machine or not.
func freshHook(t *testing.T, enroll bool) hook.Reply {
	t.Helper()
	b, _ := testBinding(t)
	root := profiledRepository(t)
	if enroll {
		data := readTestFile(t, filepath.Join(root, "golaw.yml"))
		p, err := b.compile(data, now())
		if err == nil {
			err = b.enroll(root, data, p, now())
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	agent := agentUnderTest()
	deps, err := b.hookDependencies(agent, now())
	if err != nil {
		t.Fatal(err)
	}
	event := eventAt(t, "Write", root, map[string]any{"file_path": filepath.Join(root, "notes.md"), "content": "notes\n"})
	return hook.Run(t.Context(), agent.ID, strings.NewReader(event), agent.Adapter, deps)
}

func TestFreshMachineHookDeniesAnEnrolledRepositoryAndAllowsAnUnenrolledOne(t *testing.T) {
	t.Parallel()
	human := "Perfil " + profiletest.Name + " ausente ou divergente: execute golaw profile install " + profiletest.Source + "@" + profiletest.Version
	said := string(freshHook(t, true).Stdout)
	if !strings.Contains(said, `"permissionDecision":"deny"`) || !strings.Contains(said, fixtureInstallCommand) || !strings.Contains(said, human) {
		t.Errorf("an enrolled repository without the bundle: %s", said)
	}
	if reply := freshHook(t, false); len(reply.Stdout) != 0 || reply.ExitCode != 0 {
		t.Errorf("an unenrolled location: exit %d, stdout %s", reply.ExitCode, reply.Stdout)
	}
}

func TestFreshMachineDeniesNothingElse(t *testing.T) {
	t.Parallel()
	root := profiledRepository(t)
	ta := freshMachineApp(t)
	if code := ta.run(t.Context(), []string{"version"}); code != exitClean {
		t.Errorf("version: exit %d, stderr %q", code, ta.stderr.String())
	}
	line := ta.bind().profileDoctor(t.Context(), root, now())
	if line.State != "red" || line.Code != "profile-not-installed" || !strings.Contains(line.Detail, fixtureInstallCommand) {
		t.Errorf("doctor profile line = %+v", line)
	}
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	profiletest.Install(t, dirs.Config)
	ta.run(t.Context(), []string{"scan", root})
	if out := ta.stdout.String() + ta.stderr.String(); strings.Contains(out, "golaw profile install") {
		t.Errorf("the installed bundle still names the install command:\n%s", out)
	}
}

// rungLoad answers what the load of an armed rung reads over a repository
// whose golaw.yml pins the fixture, with the bundle installed or tampered.
func rungLoad(t *testing.T, tampered bool) (*policy.Loaded, error) {
	t.Helper()
	b, _ := testBinding(t)
	dirs, err := b.dirs()
	if err != nil {
		t.Fatal(err)
	}
	if tampered {
		profiletest.Tampered(t, dirs.Config)
	} else {
		profiletest.Install(t, dirs.Config)
	}
	registry, err := checks.Registry()
	if err != nil {
		t.Fatal(err)
	}
	root := profiledRepository(t)
	deps, err := b.rungDependencies(root, now(), nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	l, err := deps.Load(t.Context(), root)
	if err != nil {
		return nil, fmt.Errorf("the rung load: %w", err)
	}
	return l, nil
}

func TestRungLoadReadsTheBundleData(t *testing.T) {
	t.Parallel()
	if _, err := rungLoad(t, true); err == nil || !strings.Contains(err.Error(), fixtureInstallCommand) {
		t.Errorf("the rung loaded a tampered bundle: %v", err)
	}
	l, err := rungLoad(t, false)
	if err != nil || l.Data == nil || !slices.Contains(l.Policy.Terms.Tools, profiletest.Data(t).Terms[0]) {
		t.Errorf("the rung load lacks the bundle data: %+v, %v", l, err)
	}
}

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// fixtureCommand is the exact command every refusal of the fixture pin names.
const fixtureCommand = "run golaw profile install " + profiletest.Source + "@" + profiletest.Version

// governedApp is the test app whose every seam a governed operation reaches
// is the production binding over its temporary home: the scans, the agent
// installer, the agent hook and config explain.
func governedApp(t *testing.T) testApp {
	t.Helper()
	ta := enrolledApp(t)
	b := ta.bind()
	ta.installOptions, ta.receiptPath, ta.enrollments = b.installOptions, b.receiptPath, b.enrollments
	ta.hookDeps = b.hookDependencies
	ta.explain = b.explain
	return ta
}

// bundleStates leave the config directory of a binding in one of the states
// of D8 and answer the end of the one line every entrance prints for it.
func bundleStates() map[string]func(t *testing.T, config string) string {
	return map[string]func(t *testing.T, config string) string{
		"missing": func(*testing.T, string) string {
			return "profile fixture is not installed: " + fixtureCommand
		},
		"files changed": func(t *testing.T, config string) string {
			t.Helper()
			profiletest.Tampered(t, config)
			return "differs from its stamp " + profiletest.Ref(t).Digest + ": " + fixtureCommand
		},
		"another version": func(t *testing.T, config string) string {
			t.Helper()
			profiletest.Install(t, config)
			stamp := strings.Replace(stampOf(t, config), "version: 1.2.3", "version: 1.3.0", 1)
			if err := os.WriteFile(policy.StampPath(config, profiletest.Name), []byte(stamp), 0o600); err != nil {
				t.Fatal(err)
			}
			return "profile fixture: version is 1.2.3 in golaw.yml and 1.3.0 in the installed bundle: " + fixtureCommand
		},
	}
}

// stampOf reads the stamp of the fixture's current bundle as text.
func stampOf(t *testing.T, config string) string {
	t.Helper()
	data, err := os.ReadFile(policy.StampPath(config, profiletest.Name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestEveryEntranceRefusesWithTheInstallCommand pins A19 and D8 on the
// governed operations that load the policy: one line on stderr naming the
// exact command, exit 2, nothing on stdout, for every state of the bundle.
func TestEveryEntranceRefusesWithTheInstallCommand(t *testing.T) {
	t.Parallel()
	entrances := map[string]func(root, enrolled string) []string{
		"scan":          func(root, _ string) []string { return []string{"scan", root} },
		"scan staged":   func(root, _ string) []string { return []string{"scan", "--staged", root} },
		"scan enrolled": func(root, enrolled string) []string { return []string{"scan", "--enrolled", enrolled, root} },
		"agent install": func(root, _ string) []string { return []string{"agent", "install", "--agent", agents.IDs()[0], root} },
		"setup":         func(root, _ string) []string { return []string{"setup", root} },
		"coverage": func(root, _ string) []string {
			return []string{"coverage", "--input", filepath.Join(root, "cover.out"), root}
		},
		"config explain": func(root, _ string) []string { return []string{"config", "explain", root} },
	}
	for state, prepare := range bundleStates() {
		for entrance, args := range entrances {
			t.Run(state+"/"+entrance, func(t *testing.T) {
				t.Parallel()
				requireOneRefusal(t, prepare, args)
			})
		}
	}
}

// requireOneRefusal runs one entrance over a fresh profiled repository with
// the bundle of the binding in a state, and asserts the one line of D8.
func requireOneRefusal(t *testing.T, prepare func(*testing.T, string) string, args func(root, enrolled string) []string) {
	t.Helper()
	root := profiledRepository(t)
	data, err := os.ReadFile(filepath.Join(root, policy.FileName))
	if err != nil {
		t.Fatal(err)
	}
	ta := governedApp(t)
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	want := prepare(t, dirs.Config)
	enrolled := digestOf(data) + "," + strings.TrimPrefix(profiletest.Ref(t).Digest, "sha256:")
	got, stderr := ta.run(t.Context(), args(root, enrolled)), ta.stderr.String()
	if got != exitFailure || ta.stdout.Len() != 0 || strings.Count(stderr, "\n") != 1 ||
		!strings.HasPrefix(stderr, "golaw: ") || !strings.HasSuffix(stderr, want+"\n") {
		t.Fatalf("exit %d, stdout %q, stderr %q, want one line ending with %q", got, ta.stdout.String(), stderr, want)
	}
}

// TestAnInstalledBundleRefusesNothing is the near miss of every entrance:
// with the pinned bundle installed no load refuses and nothing names the
// install command.
func TestAnInstalledBundleRefusesNothing(t *testing.T) {
	t.Parallel()
	root := profiledRepository(t)
	ta := governedApp(t)
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	profiletest.Install(t, dirs.Config)
	if _, err := ta.bind().load(t.Context(), root, now()); err != nil {
		t.Fatalf("load = %v", err)
	}
	if got := ta.run(t.Context(), []string{"scan", root}); got == exitFailure && ta.stderr.Len() != 0 || strings.Contains(ta.stderr.String(), "profile install") {
		t.Fatalf("exit %d, stderr %q", got, ta.stderr.String())
	}
}

// TestTheHookDeniesAnEnrolledRepositoryAndAllowsAnUnenrolledOne pins the
// agent hook of D8: an enrolled repository whose pinned bundle is missing
// is denied with the English reason and the pt-BR line naming the command,
// and the same tree unenrolled keeps the valid allow of A3, so a fresh
// clone never bricks.
func TestTheHookDeniesAnEnrolledRepositoryAndAllowsAnUnenrolledOne(t *testing.T) {
	t.Parallel()
	root := profiledRepository(t)
	data, err := os.ReadFile(filepath.Join(root, policy.FileName))
	if err != nil {
		t.Fatal(err)
	}
	enrolled := governedApp(t)
	dirs, err := enrolled.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	record := policy.Record{
		Root: root, PolicyDigest: digestOf(data), ProfileDigest: profiletest.Ref(t).Digest,
		KitVersion: testVersion, WrittenBy: policy.WrittenByInit, WrittenAt: now(),
	}
	if err := policy.WriteRecord(dirs.State, record); err != nil {
		t.Fatal(err)
	}
	enrolled.stdin = strings.NewReader(writeEvent(t, root, "content\n"))
	if got := enrolled.run(t.Context(), []string{"hook", agents.IDs()[0]}); got != exitClean {
		t.Fatalf("enrolled hook exit = %d, stderr %q", got, enrolled.stderr.String())
	}
	var answer struct {
		Output struct {
			Decision string `json:"permissionDecision"`
			Reason   string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
		Human string `json:"systemMessage"`
	}
	if err := json.Unmarshal(enrolled.stdout.Bytes(), &answer); err != nil {
		t.Fatalf("hook output %q: %v", enrolled.stdout.String(), err)
	}
	if answer.Output.Decision != "deny" || !strings.HasSuffix(answer.Output.Reason, "profile fixture is not installed: "+fixtureCommand) ||
		answer.Human != "Perfil fixture ausente ou divergente: execute golaw profile install "+profiletest.Source+"@"+profiletest.Version {
		t.Fatalf("hook answer = %+v", answer)
	}
	unenrolled := governedApp(t)
	unenrolled.stdin = strings.NewReader(writeEvent(t, root, "content\n"))
	if got := unenrolled.run(t.Context(), []string{"hook", agents.IDs()[0]}); got != exitClean || unenrolled.stdout.Len() != 0 || unenrolled.stderr.Len() != 0 {
		t.Fatalf("unenrolled hook: exit %d, stdout %q, stderr %q", got, unenrolled.stdout.String(), unenrolled.stderr.String())
	}
}

// TestTheHookDeniesTheInstallCommandItself pins the answer to the agent's
// own run of the printed command while the bundle of an enrolled
// repository is missing: the guard cannot load the policy that would judge
// it, so it denies it as every other event of that repository, with the
// same reason and line, and the person runs the command in a terminal.
func TestTheHookDeniesTheInstallCommandItself(t *testing.T) {
	t.Parallel()
	root := profiledRepository(t)
	data, err := os.ReadFile(filepath.Join(root, policy.FileName))
	if err != nil {
		t.Fatal(err)
	}
	ta := governedApp(t)
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	record := policy.Record{
		Root: root, PolicyDigest: digestOf(data), ProfileDigest: profiletest.Ref(t).Digest,
		KitVersion: testVersion, WrittenBy: policy.WrittenByInit, WrittenAt: now(),
	}
	if err := policy.WriteRecord(dirs.State, record); err != nil {
		t.Fatal(err)
	}
	command := "golaw profile install " + profiletest.Source + "@" + profiletest.Version
	ta.stdin = strings.NewReader(eventAt(t, "Bash", root, map[string]any{"command": command}))
	if got := ta.run(t.Context(), []string{"hook", agents.IDs()[0]}); got != exitClean {
		t.Fatalf("hook exit = %d, stderr %q", got, ta.stderr.String())
	}
	var answer struct {
		Output struct {
			Decision string `json:"permissionDecision"`
			Reason   string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
		Human string `json:"systemMessage"`
	}
	if err := json.Unmarshal(ta.stdout.Bytes(), &answer); err != nil {
		t.Fatalf("hook output %q: %v", ta.stdout.String(), err)
	}
	if answer.Output.Decision != "deny" || !strings.HasSuffix(answer.Output.Reason, fixtureCommand) || answer.Human != "Perfil fixture ausente ou divergente: execute "+command {
		t.Fatalf("hook answer = %+v", answer)
	}
}

// TestAnArmedRungRefusesWithTheInstallCommand pins the rung of D8: the
// pre-commit rung golaw setup armed names the command once and exits 2.
//
//nolint:paralleltest // the rung runs in the directory of the repository, which t.Chdir sets
func TestAnArmedRungRefusesWithTheInstallCommand(t *testing.T) {
	root := profiledRepository(t)
	ta := rungApp(t, root)
	got, out := ta.run(t.Context(), []string{"hook", "pre-commit"}), ta.stdout.String()
	if got != exitFailure || strings.Count(out, fixtureCommand) != 1 || !strings.Contains(out, "profile fixture is not installed: "+fixtureCommand+"\n") {
		t.Fatalf("exit %d, stdout %q, stderr %q", got, out, ta.stderr.String())
	}
}

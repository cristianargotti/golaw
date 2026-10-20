package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/setup"
)

// collectCase is one setup state doctor collects: the receipt state, the
// gitleaks version installed, the doctor line and state it answers, and
// whether gitleaks is missing or the routing changed.
type collectCase struct {
	name, receipt, version, id, want string
	missing, routing                 bool
}

// collectFixture is what a collect row starts from: the repository, its
// hooks, the state and tool directories, the receipt and the fake tools.
type collectFixture struct {
	repo        *gittest.Repo
	root, hooks string
	state, bin  string
	receipt     setup.Receipt
	receiptPath string
	installed   map[string]string
}

//nolint:paralleltest // Each row installs a private executable search path.
func TestCollectSetupRetainsLiveEvidence(t *testing.T) {
	for _, tc := range []collectCase{
		{"matching", "present", "8.30.1", "doctor.setup-tools", "ok", false, false},
		{"missing tool", "present", "", "doctor.setup-tools", "missing", true, false},
		{"another version", "present", "8.30.0", "doctor.setup-tools", "version", false, false},
		{"changed hooks path", "present", "8.30.1", "doctor.setup-routing", "drift", false, true},
		{"absent receipt", "absent", "8.30.1", "doctor.setup-receipt", "not-armed", false, false},
		{"malformed receipt", "invalid", "8.30.1", "doctor.setup-receipt", "invalid", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCollectFixture(t, tc)
			facts := collectRow(t, f, tc)
			if tc.receipt != "present" {
				requireNoProbes(t, facts, f)
			} else {
				requireLiveEvidence(t, facts, f, tc)
			}
			requireSetupLine(t, facts, tc)
		})
	}
}

// newCollectFixture is the repository of a row with its three recorded
// hooks, the pinned tools faked in a private directory, gitleaks at the
// version of the row or missing, the receipt that records them in the
// state the row names, and the routing of the row.
func newCollectFixture(t *testing.T, tc collectCase) collectFixture {
	t.Helper()
	repo := gittest.New(t)
	root, identity, err := install.ProjectIdentity(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	f := collectFixture{
		repo: repo, root: root, hooks: filepath.Join(repo.GitDir(), "hooks"),
		state: t.TempDir(), bin: t.TempDir(), installed: make(map[string]string),
	}
	f.receipt = setup.Receipt{
		Schema: setup.ReceiptSchema, Root: root, Identity: identity,
		Version: "0.2.0", HooksPath: f.hooks,
		ArmedAt: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC),
	}
	recordHooks(t, &f)
	installTools(t, &f, tc)
	f.receiptPath = setup.ReceiptPath(f.state, identity)
	writeCollectReceipt(t, f, tc.receipt)
	if tc.routing {
		repo.Git("config", "--local", "core.hooksPath", "custom-hooks")
	}
	return f
}

// recordHooks writes the three hooks the receipt records, with their
// hashes.
func recordHooks(t *testing.T, f *collectFixture) {
	t.Helper()
	for _, name := range []string{"pre-commit", "commit-msg", "pre-push"} {
		path := filepath.Join(f.hooks, name)
		data := []byte("recorded hook\n")
		if err := os.WriteFile(path, data, 0o700); err != nil {
			t.Fatal(err)
		}
		f.receipt.Hooks = append(f.receipt.Hooks, install.Artifact{
			Path: path, SHA256: install.Digest(data),
		})
	}
}

// installTools fakes every pinned tool in the private directory, gitleaks
// at the version of the row or not at all, and records each in the receipt
// at its pin.
func installTools(t *testing.T, f *collectFixture, tc collectCase) {
	t.Helper()
	for _, tool := range runner.Toolchain() {
		version := tool.Version
		if tool.Name == "gitleaks" {
			if tc.missing {
				continue
			}
			version = tc.version
		}
		path := runnertest.Install(t, f.bin, tool.Name, runnertest.Behavior{
			Probe: tool.Probe, Version: version,
		})
		f.installed[tool.Name] = path
		f.receipt.Tools = append(f.receipt.Tools, setup.ToolRecord{
			Name: tool.Name, Version: tool.Version, Path: path,
		})
	}
}

// writeCollectReceipt writes the receipt of the fixture, a receipt that
// does not decode, or none, as the row names.
func writeCollectReceipt(t *testing.T, f collectFixture, state string) {
	t.Helper()
	switch state {
	case "present":
		if err := setup.WriteReceipt(f.receiptPath, f.receipt); err != nil {
			t.Fatal(err)
		}
	case "invalid":
		if err := os.MkdirAll(filepath.Dir(f.receiptPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f.receiptPath, []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// collectRow collects the setup facts under the PATH of the row, the
// private tools before git, or the private tools alone when gitleaks is
// missing, and requires the receipt evidence.
func collectRow(t *testing.T, f collectFixture, tc collectCase) SetupFacts {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	search := f.bin + string(os.PathListSeparator) + filepath.Dir(git)
	if tc.missing {
		search = f.bin
	}
	t.Setenv("PATH", search)
	ladder := model.Ladder{PreCommit: true, CommitMsg: true, PrePush: true}
	facts, err := CollectSetup(t.Context(), f.root, f.state, gittest.Isolated(), ladder)
	if err != nil {
		t.Fatal(err)
	}
	if facts.ReceiptPath != f.receiptPath || facts.ReceiptState != tc.receipt {
		t.Fatalf("receipt evidence = %+v", facts)
	}
	return facts
}

// requireNoProbes requires an absent or invalid receipt to probe no hook
// and no tool.
func requireNoProbes(t *testing.T, facts SetupFacts, f collectFixture) {
	t.Helper()
	if len(facts.Hooks) != 0 || len(facts.Tools) != 0 {
		t.Fatalf("untrusted receipt triggered probes: %+v", facts)
	}
	for name := range f.installed {
		if calls := runnertest.Calls(t, f.bin, name); len(calls) != 0 {
			t.Fatalf("unarmed probe %s: %v", name, calls)
		}
	}
}

// requireLiveEvidence requires a present receipt to yield the three hooks
// at their hashes, the effective hooks directory when gitleaks is there,
// and the probes of the tools.
func requireLiveEvidence(t *testing.T, facts SetupFacts, f collectFixture, tc collectCase) {
	t.Helper()
	if len(facts.Hooks) != 3 || len(facts.Tools) != len(runner.Toolchain()) {
		t.Fatalf("incomplete evidence: %+v", facts)
	}
	for _, hook := range facts.Hooks {
		if hook.Error != "" || hook.SHA256 != install.Digest([]byte("recorded hook\n")) {
			t.Fatalf("hook evidence = %+v", hook)
		}
	}
	if !tc.missing {
		requireEffectiveHooks(t, facts, f, tc.routing)
	}
	requireProbedTools(t, facts, f)
}

// requireEffectiveHooks requires the effective hooks directory, the one the
// changed routing names when the row changes it.
func requireEffectiveHooks(t *testing.T, facts SetupFacts, f collectFixture, routing bool) {
	t.Helper()
	expected := f.hooks
	if routing {
		expected = filepath.Join(f.root, "custom-hooks")
	}
	if filepath.Clean(facts.EffectiveHooksPath) != filepath.Clean(expected) {
		t.Fatalf("effective hooks = %q, want %q", facts.EffectiveHooksPath, expected)
	}
}

// requireProbedTools requires every tool but govulncheck probed, and
// govulncheck never started but found on PATH at the version its receipt
// records.
func requireProbedTools(t *testing.T, facts SetupFacts, f collectFixture) {
	t.Helper()
	for name := range f.installed {
		calls := runnertest.Calls(t, f.bin, name)
		if name == "govulncheck" && len(calls) != 0 {
			t.Fatalf("doctor started govulncheck, whose probe asks the vulnerability database: %v", calls)
		}
		if name != "govulncheck" && len(calls) == 0 {
			t.Fatalf("tool was not probed: %s", name)
		}
	}
	for _, status := range facts.Tools {
		if status.Name == "govulncheck" && (status.Path != f.installed["govulncheck"] || status.Version != "1.7.0") {
			t.Fatalf("govulncheck evidence = %+v", status)
		}
	}
}

// requireSetupLine requires the doctor line and state of the row, for the
// tool line the one of gitleaks.
func requireSetupLine(t *testing.T, facts SetupFacts, tc collectCase) {
	t.Helper()
	lines := AssessSetup(facts)
	for _, line := range lines {
		if line.ID == tc.id && line.State == tc.want &&
			(tc.id != "doctor.setup-tools" || line.Key == "gitleaks") {
			return
		}
	}
	t.Fatalf("missing %s %s: %+v", tc.id, tc.want, lines)
}

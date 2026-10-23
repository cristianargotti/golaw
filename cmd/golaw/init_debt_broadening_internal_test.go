package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// adoptionDir is the fixture of the seeded adoption the M5 rows read: the
// repository with its debt under debt/ and the measurements under inputs/.
func adoptionDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(profiletest.Dir(t), "..", "..", "..", "..", "tools", "acceptance", "testdata", "m5", "adoption")
}

// debtFixture reads the files of the debt repository under the adoption
// fixture, the .txt suffix of each name dropped.
func debtFixture(t *testing.T, dir string) map[string]string {
	t.Helper()
	debt := os.DirFS(filepath.Join(dir, "debt"))
	files := map[string]string{}
	err := fs.WalkDir(debt, ".", func(rel string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || !entry.Type().IsRegular() {
			return walkErr
		}
		data, readErr := fs.ReadFile(debt, rel)
		if readErr != nil {
			t.Fatal(readErr)
		}
		files[strings.TrimSuffix(rel, ".txt")] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// seededAdoption commits the debt repository of the adoption fixture under
// dir, seeds it with the measured inputs, commits the adoption and answers
// the app that seeded it, whose binding holds the enrollment record, with
// the seeded golaw.yml. dir is read before any test changes directory.
func seededAdoption(t *testing.T, dir string) (testApp, *gittest.Repo, string) {
	t.Helper()
	repo := gittest.New(t)
	repo.Commit("chore: the repository with its debt", debtFixture(t, dir))
	inputs := filepath.Join(dir, "inputs")
	ta := seedApp(t)
	args := []string{
		"init", "--apply", "--seed-debt", "--coverage-profile", filepath.Join(inputs, "profile.cov.txt"),
		"--vulnerabilities", filepath.Join(inputs, "govulncheck.json.txt"), repo.Root,
	}
	if code := ta.run(t.Context(), args); code != exitClean {
		t.Fatalf("seed exit = %d: %s", code, ta.stderr.String())
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "build: adopt golaw with its debt")
	seeded := policyText(t, repo.Root)
	for _, want := range []string{`path: "long.go"`, `file_exemptions: ["long.go"]`, `id: "GO-2026-0002"`} {
		if !strings.Contains(seeded, want) {
			t.Fatalf("the seeded policy lacks %q:\n%s", want, seeded)
		}
	}
	ta.stdout.Reset()
	ta.stderr.Reset()
	return ta, repo, seeded
}

// broadening is one edit of the seeded golaw.yml: red when it adds or
// broadens an entry (A24), near when it only removes or shortens one.
type broadening struct {
	name  string
	red   bool
	named string // the key the refusal names
	edit  func(seeded string) string
}

// replaceLine swaps the first line of text that holds old for the line
// with new in its place; add keeps the original line after the new one.
func replaceLine(text, old, replacement string, add bool) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.Contains(line, old) {
			changed := strings.Replace(line, old, replacement, 1)
			if add {
				changed += "\n" + line
			}
			lines[i] = changed
			return strings.Join(lines, "\n")
		}
	}
	return text
}

func broadenings() []broadening {
	later, earlier := `until: "2027-05-12"`, `until: "2027-01-12"`
	return []broadening{
		{"red an added exception", true, "exceptions", func(s string) string { return replaceLine(s, `path: "long.go"`, `path: "other.go"`, true) }},
		{"red a later exception", true, "exceptions", func(s string) string { return replaceLine(s, `until: "`+seedUntil+`" }`, later+" }", false) }},
		{"red an added waiver", true, "vulnerability_waivers", func(s string) string { return replaceLine(s, `"GO-2026-0002"`, `"GO-2026-0009"`, true) }},
		{"red a later waiver", true, "vulnerability_waivers", func(s string) string {
			return replaceLine(s, `until: "`+seedUntil+`", record`, later+", record", false)
		}},
		{"red an added exemption", true, "checks.coverage.file_exemptions", func(s string) string {
			return strings.Replace(s, `file_exemptions: ["long.go"]`, `file_exemptions: ["long.go", "other.go"]`, 1)
		}},
		{"near an exemption removed", false, "", func(s string) string {
			return strings.Replace(s, `file_exemptions: ["long.go"]`, `file_exemptions: []`, 1)
		}},
		{"near an earlier expiry", false, "", func(s string) string { return strings.ReplaceAll(s, `until: "`+seedUntil+`"`, earlier) }},
		{"near another record", false, "", func(s string) string {
			return strings.Replace(s, `record: "`+seedRecord+`" }`, `record: "docs/adr/0003-debt.md" }`, 1)
		}},
	}
}

// requireBroadening requires a red edit to be refused naming its key and a
// near edit to pass the entrance clean, from the exit code and the output.
func requireBroadening(t *testing.T, b broadening, code int, out string) {
	t.Helper()
	refused := code == exitFailure && strings.Contains(out, "weakens") && strings.Contains(out, b.named)
	if b.red != refused || !b.red && code != exitClean {
		t.Errorf("%s: exit %d:\n%s", b.name, code, out)
	}
}

func TestStagedBroadeningOfTheSeedIsRefused(t *testing.T) {
	t.Parallel()
	for _, b := range broadenings() {
		t.Run(b.name, func(t *testing.T) {
			t.Parallel()
			ta, repo, seeded := seededAdoption(t, adoptionDir(t))
			repo.Stage(map[string]string{bootstrap.PolicyFile: b.edit(seeded)})
			code := ta.run(t.Context(), []string{"scan", "--staged", repo.Root})
			requireBroadening(t, b, code, ta.stderr.String())
			if b.red && !strings.Contains(ta.stderr.String(), "git restore --staged golaw.yml") {
				t.Errorf("the refusal names no fix: %s", ta.stderr.String())
			}
		})
	}
}

//nolint:paralleltest // the rung runs in the directory of the repository, which t.Chdir sets
func TestPreCommitRungRefusesAStagedBroadening(t *testing.T) {
	dir := adoptionDir(t)
	for _, b := range broadenings()[:2] {
		ta, repo, seeded := seededAdoption(t, dir)
		armed(t, ta, repo.Root)
		t.Chdir(repo.Root)
		repo.Stage(map[string]string{bootstrap.PolicyFile: b.edit(seeded)})
		code := ta.run(t.Context(), []string{"hook", "pre-commit"})
		requireBroadening(t, b, code, ta.stdout.String())
		if !strings.HasPrefix(ta.stdout.String(), "golaw pre-commit: staged scan: staged golaw.yml: ") {
			t.Errorf("%s: stdout = %q", b.name, ta.stdout.String())
		}
	}
	near := broadenings()[5]
	ta, repo, seeded := seededAdoption(t, dir)
	armed(t, ta, repo.Root)
	t.Chdir(repo.Root)
	repo.Stage(map[string]string{bootstrap.PolicyFile: near.edit(seeded)})
	ta.run(t.Context(), []string{"hook", "pre-commit"})
	if strings.Contains(ta.stdout.String(), "weakens") {
		t.Errorf("a shrink was refused as a broadening: %s", ta.stdout.String())
	}
}

func TestInitUpdateRefusesABroadenedCheckout(t *testing.T) {
	t.Parallel()
	for _, b := range broadenings() {
		t.Run(b.name, func(t *testing.T) {
			t.Parallel()
			ta, repo, seeded := seededAdoption(t, adoptionDir(t))
			repo.Write(bootstrap.PolicyFile, b.edit(seeded))
			code := ta.run(t.Context(), []string{"init", "--update", "--apply", repo.Root})
			requireBroadening(t, b, code, ta.stderr.String())
		})
	}
}

func TestAgentInstallRefusesACommittedBroadening(t *testing.T) {
	t.Parallel()
	for _, b := range broadenings() {
		t.Run(b.name, func(t *testing.T) {
			t.Parallel()
			ta, repo, seeded := seededAdoption(t, adoptionDir(t))
			repo.Commit("build: edit the debt set", map[string]string{bootstrap.PolicyFile: b.edit(seeded)})
			bind := ta.bind()
			ta.installOptions = bind.installOptions
			ta.receiptPath = bind.receiptPath
			ta.enrollments = bind.enrollments
			code := ta.run(t.Context(), []string{"agent", "install", "--agent", agents.IDs()[0], repo.Root})
			requireBroadening(t, b, code, ta.stderr.String())
		})
	}
}

func TestScanEnrolledRefusesACommittedBroadeningThePipelineDidNotProvision(t *testing.T) {
	t.Parallel()
	for _, b := range broadenings() {
		t.Run(b.name, func(t *testing.T) {
			t.Parallel()
			ta, repo, seeded := seededAdoption(t, adoptionDir(t))
			edited := b.edit(seeded)
			repo.Commit("build: edit the debt set", map[string]string{bootstrap.PolicyFile: edited})
			provisioned := digestOf([]byte(seeded))
			if !b.red {
				provisioned = digestOf([]byte(edited))
			}
			code := ta.run(t.Context(), []string{"scan", "--enrolled", provisioned, repo.Root})
			if refused := code == exitFailure && strings.Contains(ta.stderr.String(), digestOf([]byte(edited))); refused != b.red {
				t.Errorf("exit %d, stderr %q", code, ta.stderr.String())
			}
		})
	}
}

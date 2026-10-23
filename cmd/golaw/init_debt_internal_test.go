package main

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/policy"
)

// The record and the expiry the seed writes on the clock of the test app,
// 2026-10-14 plus the 180 days of the horizon.
const (
	seedRecord = "docs/adr/0002-inherited-debt.md"
	seedUntil  = "2027-04-12"
)

// seedStream is a govulncheck stream with two advisories the code reaches
// and one it only depends on.
const seedStream = `{"config": {"scanner_name": "govulncheck", "scanner_version": "v1.7.0"}}
{"finding": {"osv": "GO-2026-0001", "fixed_version": "v1.27.2", "trace": [{"module": "stdlib", "version": "v1.27.0"}]}}
{"finding": {"osv": "GO-2026-0003", "trace": [{"module": "example.test/dep", "package": "example.test/dep", "function": "Do"}]}}
{"finding": {"osv": "GO-2026-0002", "fixed_version": "v1.27.2", "trace": [{"module": "stdlib", "package": "net/http", "function": "Serve"}]}}
`

// seedProfile is a cover profile with a.go uncovered and b.go covered,
// each over the 20 statement minimum.
const seedProfile = "mode: set\nexample.test/seed/a.go:3.16,3.26 25 0\nexample.test/seed/b.go:3.16,3.26 25 1\n"

// seededOnceText is the refusal of a second seed as init prints it.
const seededOnceText = "init: usage: the debt set is seeded once (A24); remove entries from golaw.yml, never add them"

// seedRefusal is one input the seed refuses, with the text it prints;
// setup prepares the repository before init runs.
type seedRefusal struct {
	name  string
	args  []string
	setup func(t *testing.T, ta testApp, repo *gittest.Repo)
	want  string
}

// seedApp is the init app of the production binding with the catalog, the
// worktree snapshot, the load and the report, so init seeds and scan reads
// what it seeded.
func seedApp(t *testing.T) testApp {
	t.Helper()
	ta := initBindApp(t)
	b := ta.bind()
	ta.registry = func() (*engine.Registry, error) { return checks.Registry() }
	ta.snapshot, ta.load, ta.render = b.snapshot, b.load, b.render
	return ta
}

// scanSeeded commits what init --apply wrote and scans the repository with
// the binding of the app that seeded it, so the record it enrolled holds.
func scanSeeded(t *testing.T, ta testApp, repo *gittest.Repo) string {
	t.Helper()
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "build: adopt golaw")
	scan := seedApp(t)
	b := ta.bind()
	scan.bind = ta.bind
	scan.snapshot, scan.load, scan.render = b.snapshot, b.load, b.render
	scan.run(t.Context(), []string{"scan", repo.Root})
	return scan.stdout.String() + scan.stderr.String()
}

// seedRepository commits a module with the files given and answers it.
func seedRepository(t *testing.T, files map[string]string) *gittest.Repo {
	t.Helper()
	repo := gittest.New(t)
	all := map[string]string{
		"go.mod": "module example.test/seed\n\ngo 1.27\n",
		"a.go":   "package seed\n\nfunc A() int { return 1 }\n",
		"b.go":   "package seed\n\nfunc B() int { return 2 }\n",
	}
	maps.Copy(all, files)
	repo.Commit("chore: start the module", all)
	return repo
}

// seedInputs writes the cover profile and the govulncheck stream outside
// the repository and answers their paths.
func seedInputs(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	profile, stream := filepath.Join(dir, "profile.cov"), filepath.Join(dir, "vuln.json")
	for path, content := range map[string]string{profile: seedProfile, stream: seedStream} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return profile, stream
}

// policyText is golaw.yml of a root, empty when the root holds none.
func policyText(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, bootstrap.PolicyFile))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	return string(data)
}

// seededPolicy decodes the golaw.yml of a root.
func seededPolicy(t *testing.T, root string) policy.File {
	t.Helper()
	f, err := policy.Decode([]byte(policyText(t, root)))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// requireSeedPreview requires init --seed-debt without --apply to print the
// seeded line of golaw.yml, the record and the count, and to write nothing.
func requireSeedPreview(t *testing.T, ta testApp, root string) {
	t.Helper()
	if code := ta.run(t.Context(), []string{"init", "--seed-debt", root}); code != exitClean {
		t.Fatalf("preview exit = %d: %s", code, ta.stderr.String())
	}
	for _, want := range []string{
		`+  - { check: "text.dashes", path: "notes.md", record: "` + seedRecord + `", until: "` + seedUntil + `" }`, "+++ " + seedRecord,
		"Dívida semeada em golaw.yml (exceções: ", "isenções de cobertura: 0, avisos de vulnerabilidade: 0), registrada em " + seedRecord + ".",
	} {
		if !strings.Contains(ta.stdout.String(), want) {
			t.Fatalf("the preview lacks %q:\n%s", want, ta.stdout.String())
		}
	}
	if policyText(t, root) != "" {
		t.Fatal("a preview wrote golaw.yml")
	}
}

// requireSeededSet requires the three lists and the record of the apply.
func requireSeededSet(t *testing.T, root string) {
	t.Helper()
	f := seededPolicy(t, root)
	if !slices.Contains(f.Exceptions, policy.ExceptionRule{Check: "text.dashes", Path: "notes.md", Record: seedRecord, Until: seedUntil}) ||
		slices.ContainsFunc(f.Exceptions, func(e policy.ExceptionRule) bool { return e.Path == bootstrap.PolicyFile || e.Path == seedRecord }) {
		t.Errorf("exceptions = %+v", f.Exceptions)
	}
	waivers := []policy.Waiver{{ID: "GO-2026-0002", Until: seedUntil, Record: seedRecord}, {ID: "GO-2026-0003", Until: seedUntil, Record: seedRecord}}
	if !slices.Equal(f.Waivers, waivers) || !slices.Equal(f.Checks.Coverage.FileExemptions, []string{"a.go"}) {
		t.Errorf("waivers = %+v, exemptions = %+v", f.Waivers, f.Checks.Coverage.FileExemptions)
	}
	record, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(seedRecord)))
	if err != nil || !strings.Contains(string(record), `| text.dashes | "notes.md" | `+seedUntil+` |`) || !strings.Contains(string(record), `| "a.go" |`) {
		t.Fatalf("record = %q, %v", record, err)
	}
}

func TestInitSeedDebtWritesTheReviewedDebtSetOnce(t *testing.T) {
	t.Parallel()
	repo := seedRepository(t, map[string]string{"notes.md": "one " + stagedDash() + " two\n"})
	profile, stream := seedInputs(t)
	ta := seedApp(t)
	requireSeedPreview(t, ta, repo.Root)
	ta.stdout.Reset()
	args := []string{"init", "--apply", "--seed-debt", "--coverage-profile", profile, "--vulnerabilities", stream, repo.Root}
	if code := ta.run(t.Context(), args); code != exitClean {
		t.Fatalf("apply exit = %d: %s", code, ta.stderr.String())
	}
	requireSeededSet(t, repo.Root)
	if out := scanSeeded(t, ta, repo); !strings.Contains(out, "note text.dashes") || !strings.Contains(out, "(excepted by "+seedRecord+")") {
		t.Errorf("the seeded finding is not excepted:\n%s", out)
	}
	if code := ta.run(t.Context(), []string{"init", "--apply", "--seed-debt", repo.Root}); code != exitFailure || !strings.Contains(ta.stderr.String(), seededOnceText) {
		t.Fatalf("a second seed: exit %d, stderr %q", code, ta.stderr.String())
	}
}

func TestInitSeedDebtExceptsWhatTheSeedItselfWrites(t *testing.T) {
	t.Parallel()
	word := "sub" + "agent"
	repo := seedRepository(t, map[string]string{"notes/" + word + ".md": "the " + word + " notes\n"})
	ta := seedApp(t)
	if code := ta.run(t.Context(), []string{"init", "--apply", "--seed-debt", "--language", "en", repo.Root}); code != exitClean {
		t.Fatalf("apply exit = %d: %s", code, ta.stderr.String())
	}
	if want := "coverage exemptions: 0, vulnerability waivers: 0), recorded in " + seedRecord + "."; !strings.Contains(ta.stdout.String(), "Debt seeded into golaw.yml (exceptions: ") || !strings.Contains(ta.stdout.String(), want) {
		t.Errorf("stdout lacks %q:\n%s", want, ta.stdout.String())
	}
	f := seededPolicy(t, repo.Root)
	for _, path := range []string{"notes/" + word + ".md", bootstrap.PolicyFile, seedRecord} {
		if !slices.Contains(f.Exceptions, policy.ExceptionRule{Check: "text.trace-terms", Path: path, Record: seedRecord, Until: seedUntil}) {
			t.Errorf("no exception of text.trace-terms at %s: %+v", path, f.Exceptions)
		}
	}
	if out := scanSeeded(t, ta, repo); !strings.Contains(out, "note text.trace-terms") || strings.Contains(out, "deny text.trace-terms") {
		t.Errorf("a finding the seed wrote is not excepted:\n%s", out)
	}
}

func TestInitSeedDebtWithoutDebtWritesNoRecord(t *testing.T) {
	t.Parallel()
	for language, want := range map[string]string{
		"pt-BR": "Nenhuma dívida a semear: golaw.yml começa sem exceções, isenções ou avisos.",
		"en":    "No debt to seed: golaw.yml starts without exceptions, exemptions or waivers.",
	} {
		repo := seedRepository(t, map[string]string{"notes.md": "one - two\n"})
		ta := seedApp(t)
		if code := ta.run(t.Context(), []string{"init", "--apply", "--seed-debt", "--language", language, repo.Root}); code != exitClean {
			t.Fatalf("%s apply exit = %d: %s", language, code, ta.stderr.String())
		}
		if !strings.Contains(ta.stdout.String(), want) {
			t.Errorf("%s stdout = %q", language, ta.stdout.String())
		}
		if _, err := os.Stat(filepath.Join(repo.Root, filepath.FromSlash(seedRecord))); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("a record was written for an empty set: %v", err)
		}
		if f := seededPolicy(t, repo.Root); len(f.Exceptions) != 0 || len(f.Waivers) != 0 {
			t.Errorf("policy = %+v", f)
		}
	}
}

// enrolledBefore enrolls the rendered policy for the repository, the
// record a first adoption leaves on this machine.
func enrolledBefore(t *testing.T, ta testApp, repo *gittest.Repo) {
	t.Helper()
	data := renderedPolicy(t)
	p, err := ta.bind().compile(data, now())
	if err == nil {
		err = ta.bind().enroll(repo.Root, data, p, now())
	}
	if err != nil {
		t.Fatal(err)
	}
}

// seedRefusals are the inputs the seed refuses.
func seedRefusals(t *testing.T) []seedRefusal {
	t.Helper()
	dir := t.TempDir()
	absent, notScan := filepath.Join(dir, "absent.cov"), filepath.Join(dir, "vuln.json")
	if err := os.WriteFile(notScan, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return []seedRefusal{
		{"red with update", []string{"--update", "--seed-debt"}, nil, "init: usage: --seed-debt seeds a first adoption, while init --update keeps the policy and moves the kit pin"},
		{"red inputs without the seed", []string{"--vulnerabilities", notScan}, nil, "init: usage: --coverage-profile and --vulnerabilities feed --seed-debt; give it with them"},
		{"red policy in the checkout", []string{"--apply", "--seed-debt"}, func(t *testing.T, _ testApp, repo *gittest.Repo) {
			t.Helper()
			repo.Write(bootstrap.PolicyFile, string(renderedPolicy(t)))
		}, seededOnceText},
		{"red policy at head", []string{"--apply", "--seed-debt"}, func(t *testing.T, _ testApp, repo *gittest.Repo) {
			t.Helper()
			repo.Commit("build: the policy", map[string]string{bootstrap.PolicyFile: string(renderedPolicy(t))})
			repo.Remove(bootstrap.PolicyFile)
		}, seededOnceText},
		{"red enrolled before", []string{"--apply", "--seed-debt"}, enrolledBefore, seededOnceText},
		{"red the uncommitted policy of a plain init", []string{"--apply", "--seed-debt"}, func(t *testing.T, ta testApp, repo *gittest.Repo) {
			t.Helper()
			repo.Write(bootstrap.PolicyFile, string(renderedPolicy(t)))
			enrolledBefore(t, ta, repo)
		}, "is the uncommitted policy of a plain init --apply: delete it and "},
		{"red record left behind", []string{"--apply", "--seed-debt"}, func(t *testing.T, _ testApp, repo *gittest.Repo) {
			t.Helper()
			repo.Write(seedRecord, "# notes\n")
		}, "init: usage: the debt set is seeded once (A24): " + seedRecord + " exists"},
		// The path is quoted as Go quotes it, a Windows backslash doubled.
		{"red unreadable profile", []string{"--apply", "--seed-debt", "--coverage-profile", absent}, nil, fmt.Sprintf("open coverage profile %q", absent)},
		{"red not a scan", []string{"--apply", "--seed-debt", "--vulnerabilities", notScan}, nil, "govulncheck stream is no complete scan"},
	}
}

func TestInitSeedDebtRefusesWhatItCannotSeed(t *testing.T) {
	t.Parallel()
	for _, tc := range seedRefusals(t) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := seedRepository(t, map[string]string{"notes.md": "one " + stagedDash() + " two\n"})
			ta := seedApp(t)
			if tc.setup != nil {
				tc.setup(t, ta, repo)
			}
			before := policyText(t, repo.Root)
			if code := ta.run(t.Context(), append([]string{"init"}, append(tc.args, repo.Root)...)); code != exitFailure || !strings.Contains(ta.stderr.String(), tc.want) {
				t.Fatalf("exit %d, stderr %q, want %q", code, ta.stderr.String(), tc.want)
			}
			if after := policyText(t, repo.Root); after != before {
				t.Errorf("golaw.yml changed:\n%s", after)
			}
		})
	}
}

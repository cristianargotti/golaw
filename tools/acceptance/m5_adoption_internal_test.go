package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
)

// m5FakeEnroll stands in for the enrollment record init --apply writes:
// the real root, the policy digest and the digest of the pin.
func m5FakeEnroll(t *testing.T, c command) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(c.dir, policyFile))
	if err != nil {
		t.Fatal(err)
	}
	file, err := policy.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(c.dir)
	if err != nil {
		t.Fatal(err)
	}
	record := policy.Record{Root: root, PolicyDigest: policy.Digest(file), KitVersion: m5CandidateVersion, WrittenBy: policy.WrittenByInit, WrittenAt: m2Moment()}
	if file.Profile != nil {
		record.ProfileDigest = file.Profile.Digest
	}
	if err := policy.WriteRecord(m5TestDirs(t, c.env).State, record); err != nil {
		t.Fatal(err)
	}
}

// m5FakeAdoption stands in for init: the preview prints the pin, and the
// install of the CI lane when the bundle leaves no destination to the
// platform, and writes nothing; apply writes a new policy, or moves the
// pin of an existing one to the current stamp; under the profile the
// .github/ templates are left to the platform and named, core only they
// are written.
func m5FakeAdoption(t *testing.T, c command) result {
	t.Helper()
	profiled, apply := slices.Contains(c.args, m5ProfileFlag), slices.Contains(c.args, flagApply)
	if !apply {
		config := m5TestDirs(t, c.env).Config
		stamp, err := policy.LoadStamp(policy.StampPath(config, m5Name))
		if err != nil {
			t.Fatal(err)
		}
		out := "+profile: { name: fixture, source: " + stamp.Source + ", version: " + stamp.Version + ", digest: " + stamp.Digest + " }\n"
		rules, err := os.ReadFile(filepath.Join(profile.Dir(config, m5Name), profile.RulesFile))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(rules), m5ExcludeLine) {
			out += "O CI de .github/workflows/ci.yml roda make ci, cujo golaw scan precisa do perfil fixture: " + m5CIInstall + ", com uma credencial que leia o bundle, antes de make ci.\n"
		}
		return result{stdout: out}
	}
	path := filepath.Join(c.dir, policyFile)
	current, err := os.ReadFile(path)
	switch {
	case err == nil && profiled:
		stamp, stampErr := policy.LoadStamp(policy.StampPath(m5TestDirs(t, c.env).Config, m5Name))
		if stampErr != nil {
			t.Fatal(stampErr)
		}
		moved, setErr := bootstrap.SetProfile(current, policy.ProfileRef{Name: stamp.Name, Source: stamp.Source, Version: stamp.Version, Digest: stamp.Digest})
		if setErr != nil {
			t.Fatal(setErr)
		}
		m2Write(t, path, string(moved))
	case err != nil:
		m5FakeInit(t, c)
	}
	m5FakeEnroll(t, c)
	if profiled {
		return result{stdout: m5Excluded + " .github/dependabot.yml, .github/workflows/ci.yml\n"}
	}
	m2Write(t, filepath.Join(c.dir, ".github", "workflows", "ci.yml"), "name: CI\n")
	return result{}
}

// m5AdoptionReplies are the answers of the adoption rows.
func m5AdoptionReplies(t *testing.T) []m5Reply {
	t.Helper()
	doctorAnswer := func(t *testing.T, c command) result {
		t.Helper()
		stamp, err := policy.LoadStamp(policy.StampPath(m5TestDirs(t, c.env).Config, m5Name))
		if err != nil {
			t.Fatal(err)
		}
		return result{stdout: m5DoctorStream(t, doctor.Line{ID: m5DoctorProfile, Key: m5Name, State: "ok", Code: "profile-installed", Detail: "version " + stamp.Version})}
	}
	return []m5Reply{
		{prefix: []string{m5ProfileCommand, m5InstallSub}, answer: m5LifecycleInstall},
		{prefix: []string{commandInit}, answer: m5FakeAdoption},
		{prefix: []string{m5DoctorCommand}, answer: doctorAnswer},
		{prefix: []string{commandScan}, answer: m5LifecycleScan},
	}
}

func TestM5JudgesOfTheAdoption(t *testing.T) {
	t.Parallel()
	before := "golaw: 1\nkit: { version: 0.5.0 }\nprofile: { name: fixture, source: s, version: 1.2.3, digest: sha256:" + strings.Repeat("a", 64) + " }\nlanguage: pt-BR\n"
	moved := strings.Replace(strings.Replace(before, "1.2.3", "1.3.0", 1), strings.Repeat("a", 64), strings.Repeat("b", 64), 1)
	rows := []struct {
		name, after string
		pass        bool
	}{
		{"near version and digest", moved, true},
		{"red nothing moved", before, false},
		{"red source moved", strings.Replace(moved, "source: s,", "source: t,", 1), false},
		{"red another line moved", strings.Replace(moved, "pt-BR", "en", 1), false},
		{"red version alone", strings.Replace(before, "1.2.3", "1.3.0", 1), false},
	}
	for _, row := range rows {
		if err := m5OnlyPinMoved([]byte(before), []byte(row.after)); (err == nil) != row.pass {
			t.Errorf("%s: %v", row.name, err)
		}
	}
	excluded := result{stdout: m5Excluded + " .github/dependabot.yml, .github/workflows/ci.yml\n"}
	for _, row := range []struct {
		name string
		got  result
		pass bool
	}{
		{"near both named", excluded, true},
		{"red one named", result{stdout: m5Excluded + " .github/dependabot.yml\n"}, false},
		{"red no line", result{stdout: ".github/workflows/ci.yml\n"}, false},
		{"red refused", result{code: 2, stdout: excluded.stdout}, false},
	} {
		if err := m5JudgeExcluded(row.got); (err == nil) != row.pass {
			t.Errorf("%s: %v", row.name, err)
		}
	}
	repo := t.TempDir()
	checkedWrite(t, filepath.Join(repo, ".github", "CODEOWNERS"), "* @example-org/service-team\n")
	checkedWrite(t, filepath.Join(repo, policyFile), "golaw: 1\nmanaged:\n  - { path: golaw.mk, sha256: "+strings.Repeat("c", 64)+" }\n")
	if err := m5NoGithubWrite(repo); err != nil {
		t.Fatalf("near the birth's own CODEOWNERS: %v", err)
	}
	checkedWrite(t, filepath.Join(repo, ".github", "dependabot.yml"), "version: 2\n")
	if err := m5NoGithubWrite(repo); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("red a written template = %v", err)
	}
}

func TestM5AdoptionRowsWithASimulatedCandidate(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"init-profile-pins", "init-profile-refresh", "template-born-no-github-write", "init-profile-ci-line"} {
		c := m5AdoptionCases()[slices.Index(m5Names(m5AdoptionCases()), name)]
		s := m2TestSuite(t)
		scope := s.m5Scope(m5AdoptionCategory, name)
		scope.execute = m5Script(t, scope, m5AdoptionReplies(t)...).Execute
		if err := c.run(scope, t.Context()); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		writing := s.m5Scope(m5AdoptionCategory, name+"-writing")
		replies := append([]m5Reply{{prefix: []string{commandInit, m5ProfileFlag}, answer: func(t *testing.T, c command) result {
			t.Helper()
			m2Write(t, filepath.Join(c.dir, ".github", "dependabot.yml"), "version: 2\n")
			m2Write(t, filepath.Join(c.dir, policyFile), "golaw: 1\n")
			return result{}
		}}}, m5AdoptionReplies(t)...)
		writing.execute = m5Script(t, writing, replies...).Execute
		if err := c.run(writing, t.Context()); err == nil {
			t.Errorf("%s passed over an init that ignored the profile", name)
		}
	}
	files, err := m5GithubFiles(t.TempDir())
	if err != nil || len(files) != 0 {
		t.Fatalf("a repository without .github/ = %q, %v", files, err)
	}
}

func TestM5AdoptionRowsReportEveryFailedStep(t *testing.T) {
	t.Parallel()
	for _, c := range m5AdoptionCases()[:4] {
		m5Sweep(t, m5AdoptionCategory, c, func(t *testing.T, _ *suite) []m5Reply {
			t.Helper()
			return m5AdoptionReplies(t)
		})
	}
}

// TestM5CatalogsRequireTheAdoptionRows holds the line of m5Catalogs that
// makes -m5 and -release require the adoption rows, right after the
// lifecycle rows, which no test above reaches.
func TestM5CatalogsRequireTheAdoptionRows(t *testing.T) {
	t.Parallel()
	categories := make([]string, 0, len(m5Catalogs()))
	for _, catalog := range m5Catalogs() {
		categories = append(categories, catalog.category)
		if catalog.category == m5AdoptionCategory && !slices.Equal(m5Names(catalog.cases), m5Names(m5AdoptionCases())) {
			t.Fatalf("the adoption catalog = %q", m5Names(catalog.cases))
		}
	}
	at := slices.Index(categories, m5AdoptionCategory)
	if at < 1 || categories[at-1] != m5InstallCategory {
		t.Fatalf("the categories = %q, want %s right after %s", categories, m5AdoptionCategory, m5InstallCategory)
	}
}

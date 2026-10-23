package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
)

// m5LifecycleInstall stands in for profile install: the bundle of the tag
// made current and recorded with the URL given, the canonical source named
// because it differs, or the line of a version the record holds already.
func m5LifecycleInstall(t *testing.T, c command) result {
	t.Helper()
	arg := c.args[2]
	at := strings.LastIndex(arg, "@")
	source, tag := arg[:at], arg[at+1:]
	version := strings.TrimPrefix(tag, "v")
	dirs := m5TestDirs(t, c.env)
	record, found, err := profile.ReadRecord(dirs.State, m5Name)
	if err != nil {
		t.Fatal(err)
	}
	if found && record.Current == version {
		return result{stdout: "Perfil fixture " + version + " já estava instalado\n"}
	}
	m5FakeInstall(version)(t, c)
	stamp, err := policy.LoadStamp(policy.StampPath(dirs.Config, m5Name))
	if err != nil {
		t.Fatal(err)
	}
	installed := profile.Installed{Name: m5Name, Version: version, Digest: stamp.Digest, Source: m5Canonical, URL: source, Tag: tag}
	record = profile.Record{Schema: 1, Name: m5Name, Current: version, Versions: append(record.Versions, installed)}
	if err := profile.WriteRecord(dirs.State, record); err != nil {
		t.Fatal(err)
	}
	return result{stdout: "Perfil fixture " + version + " instalado de " + arg + ", digest " + stamp.Digest + "\nFonte canônica: " + m5Canonical + "\n"}
}

// m5LifecycleRemove stands in for profile remove: the current bundle and
// the record gone.
func m5LifecycleRemove(t *testing.T, c command) result {
	t.Helper()
	dirs := m5TestDirs(t, c.env)
	if err := os.RemoveAll(profile.Dir(dirs.Config, m5Name)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(profile.RecordPath(dirs.State, m5Name)); err != nil {
		t.Fatal(err)
	}
	return result{stdout: "Perfil fixture removido\n"}
}

// m5LifecycleScan stands in for scan: the bundle read as the loader reads
// it, its refusal on the one line of D8.
func m5LifecycleScan(t *testing.T, c command) result {
	t.Helper()
	dir := profile.Dir(m5TestDirs(t, c.env).Config, m5Name)
	if _, err := os.Stat(dir); err != nil {
		return result{code: 2, stderr: "golaw: scan: profile fixture is not installed: run " + m5InstallCommand + "\n"}
	}
	if _, err := profile.Read(dir); err != nil {
		return result{code: 2, stderr: "golaw: scan: " + err.Error() + "\n"}
	}
	return result{}
}

// m5LifecycleDoctor answers the profile line of the pin at 1.2.3: installed
// while any bundle of the name is current and installed is true.
func m5LifecycleDoctor(installed bool) func(*testing.T, command) result {
	return func(t *testing.T, c command) result {
		t.Helper()
		line := doctor.Line{
			ID: m5DoctorProfile, Key: m5Name, State: "red", Code: "profile-not-installed",
			Detail: "profile fixture is not installed: run " + m5InstallCommand,
		}
		if _, err := os.Stat(profile.Dir(m5TestDirs(t, c.env).Config, m5Name)); err == nil && installed {
			line = doctor.Line{
				ID: m5DoctorProfile, Key: m5Name, State: "ok", Code: "profile-installed",
				Detail: "version 1.2.3, digest sha256:d, 2 days since 2026-10-20",
			}
		}
		return result{stdout: m5DoctorStream(t, line)}
	}
}

// m5LifecycleReplies are the answers of the lifecycle rows; installed false
// is the defect the rows exist for, a doctor that never finds the bundle.
func m5LifecycleReplies(t *testing.T, installed bool) []m5Reply {
	t.Helper()
	rollback := func(t *testing.T, c command) result {
		t.Helper()
		m5FakeInstall(m5Version)(t, c)
		return result{stdout: "Perfil fixture voltou para 1.2.3\n"}
	}
	refresh := func(t *testing.T, c command) result {
		t.Helper()
		m5FakeInstall(m5Version)(t, c)
		return result{stdout: "Perfil fixture 1.2.3 instalado de source@v1.2.3, digest sha256:d\n"}
	}
	return []m5Reply{
		{prefix: []string{m5ProfileCommand, m5InstallSub}, answer: m5LifecycleInstall},
		{prefix: []string{m5ProfileCommand, "rollback"}, answer: rollback},
		{prefix: []string{m5ProfileCommand, "refresh"}, answer: refresh},
		{prefix: []string{m5ProfileCommand, "remove"}, answer: m5LifecycleRemove},
		{prefix: []string{commandInit}, answer: m5After(m5FakeInit, result{})},
		{prefix: []string{m5DoctorCommand}, answer: m5LifecycleDoctor(installed)},
		{prefix: []string{commandScan}, answer: m5LifecycleScan},
	}
}

func TestM5InstallRowsWithASimulatedCandidate(t *testing.T) {
	t.Parallel()
	for _, c := range m5InstallCases() {
		s := m2TestSuite(t)
		scope := s.m5Scope(m5InstallCategory, c.name)
		scope.execute = m5Script(t, scope, m5LifecycleReplies(t, true)...).Execute
		if err := c.run(scope, t.Context()); err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
		if c.name == "remove" {
			continue
		}
		blind := s.m5Scope(m5InstallCategory, c.name+"-blind")
		blind.execute = m5Script(t, blind, m5LifecycleReplies(t, false)...).Execute
		if err := c.run(blind, t.Context()); err == nil {
			t.Errorf("%s passed while doctor never found the bundle", c.name)
		}
	}
	want := []string{"install", "install-idempotent", "install-mirror", "refresh", "rollback", "remove", "pin-unchanged"}
	if got := m5Names(m5InstallCases()); !slices.Equal(got, want) {
		t.Fatalf("names = %q", got)
	}
}

func TestM5UnchangedJudgesThePinAndTheTree(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	s.execute = runCommand
	repo := filepath.Join(t.TempDir(), "repo")
	checkedWrite(t, filepath.Join(repo, policyFile), "golaw: 1\n")
	if err := s.gitSteps(t.Context(), repo, []string{commandInit, m2Quiet}, []string{gitAdd, "."}, commitArgs("test: the pin")); err != nil {
		t.Fatal(err)
	}
	setup := m5Setup{repo: repo, pin: []byte("golaw: 1\n")}
	if err := s.m5Unchanged(t.Context(), setup); err != nil {
		t.Fatalf("near the committed pin: %v", err)
	}
	checkedWrite(t, filepath.Join(repo, "untracked.txt"), "x")
	if err := s.m5Unchanged(t.Context(), setup); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("red an untracked file = %v", err)
	}
	if err := os.Remove(filepath.Join(repo, "untracked.txt")); err != nil {
		t.Fatal(err)
	}
	checkedWrite(t, filepath.Join(repo, policyFile), "golaw: 1\nlanguage: pt-BR\n")
	if err := s.m5Unchanged(t.Context(), setup); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("red a moved pin = %v", err)
	}
	for _, row := range []struct {
		got  result
		pass bool
	}{
		{result{stdout: "Perfil fixture removido\n"}, true},
		{result{stdout: "Perfil fixture voltou para 1.3.0\n"}, false},
		{result{code: 2, stdout: "Perfil fixture removido\n"}, false},
	} {
		if err := m5Printed(row.got, "Perfil fixture removido"); (err == nil) != row.pass {
			t.Errorf("printed %+v: %v", row.got, err)
		}
	}
	if digest, err := m5FixtureDigest(testRoot(t)); err != nil || !strings.HasPrefix(digest, "sha256:") {
		t.Fatalf("fixture digest %q: %v", digest, err)
	}
	if _, err := m5FixtureDigest(t.TempDir()); err == nil {
		t.Fatal("a module without the fixture answered a digest")
	}
}

func TestM5InstallRowsReportEveryFailedStep(t *testing.T) {
	t.Parallel()
	for _, c := range m5InstallCases() {
		m5Sweep(t, m5InstallCategory, c, func(t *testing.T, _ *suite) []m5Reply {
			t.Helper()
			return m5LifecycleReplies(t, true)
		})
	}
}

// m5WrongInstall is an install that gets one thing wrong: "terse" prints
// neither the URL nor the digest nor the canonical source and copies on
// every call; "newer" makes 1.3.0 current whatever the tag and records
// another URL; "copying" says a version is installed already and copies
// another bundle over it.
func m5WrongInstall(kind string) func(*testing.T, command) result {
	return func(t *testing.T, c command) result {
		t.Helper()
		switch kind {
		case "terse":
			m5FakeInstall(m5Version)(t, c)
			return result{stdout: "Perfil fixture 1.2.3 instalado\n"}
		case "copying":
			got := m5LifecycleInstall(t, c)
			if strings.Contains(got.stdout, "já estava") {
				m5FakeInstall(m5NewerVersion)(t, c)
			}
			return got
		}
		got := m5LifecycleInstall(t, c)
		m5FakeInstall(m5NewerVersion)(t, c)
		dirs := m5TestDirs(t, c.env)
		record, found, err := profile.ReadRecord(dirs.State, m5Name)
		if err != nil || !found {
			t.Fatalf("record %v %v", found, err)
		}
		record.Versions[len(record.Versions)-1].URL = "elsewhere"
		if err := profile.WriteRecord(dirs.State, record); err != nil {
			t.Fatal(err)
		}
		return got
	}
}

func TestM5InstallRowsRefuseAWrongInstall(t *testing.T) {
	t.Parallel()
	for kind, names := range map[string][]string{
		"terse":   {"install", "install-idempotent", "install-mirror"},
		"newer":   {"install", "install-mirror"},
		"copying": {"install-idempotent"},
	} {
		for _, name := range names {
			c := m5InstallCases()[slices.Index(m5Names(m5InstallCases()), name)]
			s := m2TestSuite(t)
			scope := s.m5Scope(m5InstallCategory, name+"-"+kind)
			replies := append([]m5Reply{{prefix: []string{m5ProfileCommand, m5InstallSub}, answer: m5WrongInstall(kind)}}, m5LifecycleReplies(t, true)...)
			scope.execute = m5Script(t, scope, replies...).Execute
			if err := c.run(scope, t.Context()); err == nil {
				t.Errorf("%s passed over the %s install", name, kind)
			}
		}
	}
}

// TestM5CatalogsRequireTheInstallRows holds the line of m5Catalogs that
// makes -m5 and -release require the lifecycle rows, right after the
// profile rows, which no test above reaches.
func TestM5CatalogsRequireTheInstallRows(t *testing.T) {
	t.Parallel()
	categories := make([]string, 0, len(m5Catalogs()))
	for _, catalog := range m5Catalogs() {
		categories = append(categories, catalog.category)
		if catalog.category == m5InstallCategory && !slices.Equal(m5Names(catalog.cases), m5Names(m5InstallCases())) {
			t.Fatalf("the install catalog = %q", m5Names(catalog.cases))
		}
	}
	at := slices.Index(categories, m5InstallCategory)
	if at < 1 || categories[at-1] != m5ProfileCategory {
		t.Fatalf("the categories = %q, want %s right after %s", categories, m5InstallCategory, m5ProfileCategory)
	}
}

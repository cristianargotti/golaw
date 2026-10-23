package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/profile"
)

// The tests below give the lifecycle rows the wrong candidates the
// simulated one never plays, so every claim of a row is seen failing: a
// rollback that moves nothing, a remove that leaves the bundle or the
// record, and a mirror install pinned elsewhere or never recorded.

// m5WrongStep runs the row named name of the install catalog with reply
// answering first, and requires it to fail.
func m5WrongStep(t *testing.T, name, kind string, reply m5Reply) {
	t.Helper()
	cases := m5InstallCases()
	for _, c := range cases {
		if c.name != name {
			continue
		}
		s := m2TestSuite(t)
		scope := s.m5Scope(m5InstallCategory, name+"-"+kind)
		replies := append([]m5Reply{reply}, m5LifecycleReplies(t, true)...)
		scope.execute = m5Script(t, scope, replies...).Execute
		if err := c.run(scope, t.Context()); err == nil {
			t.Errorf("%s passed over %s", name, kind)
		}
		return
	}
	t.Fatalf("no install row is named %s", name)
}

func TestM5LifecycleRowsRefuseAStepThatLies(t *testing.T) {
	t.Parallel()
	removing := func(path func(dirs m5TestDirsOf) string) func(*testing.T, command) result {
		return func(t *testing.T, c command) result {
			t.Helper()
			if err := os.RemoveAll(path(m5TestDirsOf{t: t, env: c.env})); err != nil {
				t.Fatal(err)
			}
			return result{stdout: "Perfil fixture removido\n"}
		}
	}
	mirrored := func(t *testing.T, c command) result {
		t.Helper()
		m5FakeInit(t, c)
		path := filepath.Join(c.dir, policyFile)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		m2Write(t, path, strings.ReplaceAll(string(data), m5Canonical, "https://example.test/profiles/mirror"))
		return result{}
	}
	unrecorded := "Perfil fixture 1.2.3 instalado de mirror@v1.2.3, digest d\n  source: " + m5Canonical + "\n"
	for _, row := range []struct {
		name, kind string
		reply      m5Reply
	}{
		{"rollback", "a rollback that moves nothing", m5Reply{prefix: []string{m5ProfileCommand, m5RollbackSub}, answer: m5Answer(result{stdout: "Perfil fixture voltou para 1.2.3\n"})}},
		{"remove", "a remove that leaves the bundle", m5Reply{prefix: []string{m5ProfileCommand, m5RemoveSub}, answer: removing(m5TestDirsOf.record)}},
		{"remove", "a remove that leaves the record", m5Reply{prefix: []string{m5ProfileCommand, m5RemoveSub}, answer: removing(m5TestDirsOf.bundle)}},
		{"install-mirror", "an init that pins the mirror", m5Reply{prefix: []string{commandInit}, answer: mirrored}},
		{"install-mirror", "an install that records nothing", m5Reply{prefix: []string{m5ProfileCommand, m5InstallSub}, answer: m5After(m5FakeInstall(m5Version), result{stdout: unrecorded})}},
	} {
		m5WrongStep(t, row.name, row.kind, row.reply)
	}
}

// m5TestDirsOf names the paths of the fixture profile in a command's home.
type m5TestDirsOf struct {
	t   *testing.T
	env []string
}

func (d m5TestDirsOf) bundle() string { return profile.Dir(m5TestDirs(d.t, d.env).Config, m5Name) }

func (d m5TestDirsOf) record() string {
	return profile.RecordPath(m5TestDirs(d.t, d.env).State, m5Name)
}

func TestM5CurrentIsNamesTheVersionInForce(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t).m5Scope(m5InstallCategory, "current")
	m5FakeInstall(m5Version)(t, command{env: s.env})
	if err := s.m5CurrentIs(m5Version); err != nil {
		t.Fatalf("near the version installed: %v", err)
	}
	if err := s.m5CurrentIs(m5NewerVersion); err == nil {
		t.Fatal("red another version passed")
	}
}

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
)

func TestM5OneLineJudgesOneErrorWithItsRemedy(t *testing.T) {
	t.Parallel()
	refusal := "golaw: scan: invalid policy: profile fixture is not installed: run " + m5InstallCommand + "\n"
	rows := []struct {
		name string
		got  result
		pass bool
	}{
		{"near the one line on stderr", result{code: 2, stderr: refusal}, true},
		{"near the one line on stdout", result{code: 1, stdout: refusal}, true},
		{"red passed", result{code: 0, stderr: refusal}, false},
		{"red two lines", result{code: 2, stderr: refusal + refusal}, false},
		{"red no remedy", result{code: 2, stderr: "golaw: scan: invalid policy\n"}, false},
		{"red another version", result{code: 2, stderr: strings.ReplaceAll(refusal, "@1.2.3", "@9.9.9")}, false},
		{"red not run", result{err: ErrAcceptance}, false},
	}
	for _, row := range rows {
		if err := m5OneLine(row.got); (err == nil) != row.pass {
			t.Errorf("%s: %v", row.name, err)
		}
	}
}

func TestM5DoctorLineReadsEveryDocument(t *testing.T) {
	t.Parallel()
	want := doctor.Line{ID: m5DoctorProfile, Key: m5Name, State: "ok", Code: "profile-installed", Detail: "version 1.2.3"}
	got, err := m5DoctorLine(m5DoctorStream(t, want), m5DoctorProfile)
	if err != nil || got != want {
		t.Fatalf("line = %+v, %v", got, err)
	}
	if _, err := m5DoctorLine(m5DoctorStream(t), m5DoctorProfile); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("a stream without the line = %v", err)
	}
	if _, err := m5DoctorLine("{", m5DoctorProfile); err == nil || errors.Is(err, ErrAcceptance) {
		t.Fatalf("a malformed stream = %v", err)
	}
}

func TestM5RepositoryAdoptsWithAndWithoutTheProfile(t *testing.T) {
	t.Parallel()
	for _, profiled := range []bool{false, true} {
		s := m2TestSuite(t)
		var calls [][]string
		record := func(t *testing.T, c command) {
			t.Helper()
			calls = append(calls, c.args)
			m5FakeInit(t, c)
		}
		s.execute = m5Script(t, s,
			m5Reply{prefix: []string{commandInit}, answer: m5After(record, result{})},
			m5Reply{prefix: []string{m5ProfileCommand, m5InstallSub}, answer: m5After(m5FakeInstall(m5Version), result{stdout: "Perfil fixture 1.2.3 instalado\n"})},
		).Execute
		if _, err := s.m5Install(t.Context(), "source.git", m5Tag); err != nil {
			t.Fatal(err)
		}
		repo, err := s.m5Repository(t.Context(), "adopted", profiled)
		if err != nil {
			t.Fatal(err)
		}
		if len(calls) != 1 || slices.Contains(calls[0], m5ProfileFlag) != profiled || !slices.Contains(calls[0], flagApply) ||
			!slices.Contains(calls[0], m5Capabilities) {
			t.Fatalf("profiled %v: init = %q", profiled, calls)
		}
		data, err := os.ReadFile(filepath.Join(repo, policyFile))
		if err != nil {
			t.Fatal(err)
		}
		file, err := policy.Decode(data)
		if err != nil || (file.Profile != nil) != profiled {
			t.Fatalf("profiled %v: policy = %+v, %v", profiled, file.Profile, err)
		}
		m5Clean(t, s, repo)
	}
}

func TestM5InstallAndDoctorStepsNameWhatFailed(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	installed := doctor.Line{ID: m5DoctorProfile, Key: m5Name, State: "ok", Code: "profile-installed", Detail: "version 1.2.3, digest d"}
	s.execute = m5Script(t, s,
		m5Reply{prefix: []string{m5ProfileCommand, m5InstallSub}, answer: m5Answer(result{code: 2, stderr: "golaw: profile install: the clone failed\n"})},
		m5Reply{prefix: []string{m5DoctorCommand}, once: true, answer: m5Answer(result{stdout: m5DoctorStream(t, installed)})},
		m5Reply{prefix: []string{m5DoctorCommand}, once: true, answer: m5Answer(result{stdout: m5DoctorStream(t, installed)})},
		m5Reply{prefix: []string{m5DoctorCommand}, answer: m5Answer(result{code: 2, stderr: "golaw: doctor: failed\n"})},
	).Execute
	if _, err := s.m5Install(t.Context(), "source.git", m5Tag); err == nil || !strings.Contains(err.Error(), m5Tag) {
		t.Fatalf("a failed install = %v", err)
	}
	if err := s.m5ExpectProfile(t.Context(), s.root, "profile-installed", "version "+m5Version); err != nil {
		t.Fatal(err)
	}
	if err := s.m5ExpectProfile(t.Context(), s.root, "profile-not-installed", m5InstallCommand); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("another code = %v", err)
	}
	if _, err := s.m5Doctor(t.Context(), s.root); err == nil {
		t.Fatal("a failed doctor was read")
	}
	dirs, err := s.m5Dirs()
	if err != nil || !strings.HasPrefix(dirs.Config, s.home) || !strings.HasPrefix(dirs.State, s.home) {
		t.Fatalf("dirs = %+v under %q: %v", dirs, s.home, err)
	}
	if _, err := s.m5Current(); err == nil {
		t.Fatal("a home without a bundle answered a stamp")
	}
}

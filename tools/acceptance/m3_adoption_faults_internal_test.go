package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	scanreport "github.com/cristianargotti/golaw/internal/report"
)

func TestM3AdoptionStopsAtEveryFailedStep(t *testing.T) {
	t.Parallel()
	adopt := func(t *testing.T, name string, n int) (*m2Script, error) {
		t.Helper()
		s := m2TestSuite(t)
		script := newM2Script(t, s, m3FailingAt(n, m3BirthAnswer(t, s)))
		s.execute = script.Execute
		return script, s.adoptionM3(t.Context(), name)
	}
	for _, name := range m3AdoptionNames() {
		script, err := adopt(t, name, 0)
		if err != nil || script.Calls == 0 {
			t.Fatalf("near %s = %v after %d commands", name, err, script.Calls)
		}
		for n := 1; n <= script.Calls; n++ {
			t.Run(fmt.Sprintf("red %s step %d", name, n), func(t *testing.T) {
				t.Parallel()
				if _, err := adopt(t, name, n); !errors.Is(err, ErrAcceptance) {
					t.Fatalf("step %d of %d = %v", n, script.Calls, err)
				}
			})
		}
	}
}

// m3CandidateReply answers every command of the candidate with one reply
// and leaves git to the fixture executor.
func m3CandidateReply(s *suite, reply func(c command) result) m2Answer {
	return func(_ context.Context, c command) (m2Reply, bool) {
		if c.program != s.binary {
			return m2Reply{}, false
		}
		return m2Reply{got: reply(c), target: true}, true
	}
}

func TestM3AdoptionNamesEachMechanismThatFails(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, row, want string
		reply           func(t *testing.T, c command) result
	}{
		{"red a preview that fails", "birth-preview", "golaw init exit 2", func(*testing.T, command) result {
			return result{code: 2, stdout: m3PreviewText}
		}},
		{"red a preview that writes", "birth-preview", "changed the birth", func(t *testing.T, c command) result {
			t.Helper()
			checkedWrite(t, filepath.Join(c.dir, "golaw.yml"), "golaw: 1\n")
			return result{stdout: m3PreviewText}
		}},
		{"red an accept that leaves the birth file", "birth-accept", "not the strict asset", func(*testing.T, command) result {
			return result{}
		}},
		{"red an accept without a policy", "birth-accept", "read acceptance file", func(t *testing.T, c command) result {
			t.Helper()
			checkedWrite(t, filepath.Join(c.dir, m3LintConfig), "linters:\n  default: standard\n")
			return result{}
		}},
		{"red an accept with a broken policy", "birth-accept", "decode golaw.yml", func(t *testing.T, c command) result {
			t.Helper()
			checkedWrite(t, filepath.Join(c.dir, m3LintConfig), "linters:\n  default: standard\n")
			checkedWrite(t, filepath.Join(c.dir, policyFile), "golaw: [\n")
			return result{}
		}},
		{"red an older go line without the floor", "birth-older-go", "does not name the floor", func(*testing.T, command) result {
			return result{}
		}},
		{"red an older go line that renders golaw.mk", "birth-older-go", "wrote golaw.mk", func(t *testing.T, c command) result {
			t.Helper()
			checkedWrite(t, filepath.Join(c.dir, m3GolawMk), "ci:\n")
			return result{stdout: "go 1.24\n"}
		}},
	} {
		s := m2TestSuite(t)
		s.execute = newM2Script(t, s, m3CandidateReply(s, func(c command) result { return tc.reply(t, c) })).Execute
		if err := s.adoptionM3(t.Context(), tc.row); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
}

func TestM3AcceptNamesAPolicyWithoutTheManagedFile(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	s.execute = newM2Script(t, s, m3CandidateReply(s, func(c command) result {
		if len(c.args) > 2 {
			m3WriteAccepted(t, c.dir)
			checkedWrite(t, filepath.Join(c.dir, m3LintConfig), "linters:\n  default: standard\n  enable: [revive]\n")
		}
		return result{}
	})).Execute
	if err := s.adoptionM3(t.Context(), "birth-accept"); !errors.Is(err, ErrAcceptance) || !strings.Contains(err.Error(), "records no managed") {
		t.Fatalf("a hash the policy does not record = %v", err)
	}
	s = m2TestSuite(t)
	s.execute = newM2Script(t, s, m3CandidateReply(s, func(c command) result {
		if len(c.args) > 2 {
			m3WriteAccepted(t, c.dir)
			return result{}
		}
		return result{stdout: "+++ .golangci.yml\n"}
	})).Execute
	if err := s.adoptionM3(t.Context(), "birth-accept"); !errors.Is(err, ErrAcceptance) || !strings.Contains(err.Error(), "proposes .golangci.yml again") {
		t.Fatalf("a second run that proposes the file again = %v", err)
	}
}

func TestM3JudgeMeasuredRefusesAnUnmeasuredScan(t *testing.T) {
	t.Parallel()
	measured := []scanreport.Status{{ID: "policy.makefile", Status: "measured"}}
	for _, tc := range []struct {
		name string
		doc  scanreport.Document
		code int
		fail bool
	}{
		{"near findings allowed", scanreport.Document{Checks: measured}, 1, false},
		{"red no check listed", scanreport.Document{}, 0, true},
		{"red an unmeasured status", scanreport.Document{Checks: []scanreport.Status{{ID: "policy.golangci", Status: "unmeasured"}}}, 2, true},
		{"red an unmeasured count", scanreport.Document{Checks: measured, Counts: scanreport.Counts{Unmeasured: 1}}, 2, true},
		{"red a scan that exits 2", scanreport.Document{Checks: measured}, 2, true},
	} {
		if err := m3JudgeMeasured(tc.doc, tc.code); (err != nil) != tc.fail || (tc.fail && !errors.Is(err, ErrAcceptance)) {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
}

func TestM3BirthCasesNeedTheirFiles(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	s.execute = func(context.Context, command) result { return result{err: ErrAcceptance} }
	empty := t.TempDir()
	if err := s.m3BirthScan(t.Context(), empty); err == nil {
		t.Fatal("a birth without a Makefile must fail")
	}
	if err := s.m3BirthOlderGo(t.Context(), empty); err == nil {
		t.Fatal("a birth without go.mod must fail")
	}
	checkedWrite(t, filepath.Join(empty, moduleFile), "module example.test/birth\n")
	if err := s.m3BirthOlderGo(t.Context(), empty); !errors.Is(err, ErrAcceptance) || !strings.Contains(err.Error(), "no go line") {
		t.Fatalf("a go.mod without a go line = %v", err)
	}
	if err := os.Mkdir(filepath.Join(empty, m3Makefile), dirMode); err != nil {
		t.Fatal(err)
	}
	if err := s.m3BirthScan(t.Context(), empty); err == nil {
		t.Fatal("a Makefile that is a directory must fail")
	}
	s.root = t.TempDir()
	if err := s.adoptionM3(t.Context(), "birth-preview"); err == nil {
		t.Fatal("a kit without the birth fixture must fail")
	}
	blocked := m2TestSuite(t)
	checkedWrite(t, filepath.Join(blocked.base, dirRepos), "a file where the repositories go\n")
	if err := blocked.adoptionM3(t.Context(), "birth-preview"); err == nil {
		t.Fatal("a birth over a file must fail")
	}
}

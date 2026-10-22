package main

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestM3CaseEditorsRefuseWhatTheyCannotEdit(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	for _, edit := range []func(string) error{m3Disable("errcheck"), m3MoveRule("deadcode", true), m3SecondBlock(true)} {
		if err := edit(repo); err == nil {
			t.Fatal("red an edit of an absent file must fail")
		}
	}
	for body, edit := range map[string]func(string) error{
		"linters:\n  disable: errcheck\n": m3Disable("errcheck"),
		"linters: [errcheck]\n":           m3Exclude("internal/", "errcheck"),
		"- linters\n":                     m3Disable("errcheck"),
	} {
		checkedWrite(t, filepath.Join(repo, m3LintFile), body)
		if err := edit(repo); !errors.Is(err, ErrAcceptance) {
			t.Fatalf("red lint configuration %q = %v", body, err)
		}
		if data, err := os.ReadFile(filepath.Join(repo, m3LintFile)); err != nil || string(data) != body {
			t.Fatalf("a refused edit rewrote %q into %q, %v", body, data, err)
		}
	}
	checkedWrite(t, filepath.Join(repo, m2EditedAsset), "ci: scan\n\nscan:\n\tgolaw scan .\n")
	if err := m3MoveRule("deadcode", true)(repo); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("red a move of an absent rule = %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(m3QualityFile))); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a refused move wrote %s: %v", m3QualityFile, err)
	}
}

func TestM3CaseEditorsLeaveTheRedShapeUnresolved(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	checkedWrite(t, filepath.Join(repo, m2EditedAsset), "ci: deadcode\n\ndeadcode:\n\tgo test ./test/deadcode/\n\nscan:\n\tgolaw scan .\n")
	checkedWrite(t, filepath.Join(repo, moduleFile), "module example.test/service\n\ngo 1.27\n")
	for _, edit := range []func(string) error{m3MoveRule("deadcode", false), m3SecondBlock(false)} {
		if err := edit(repo); err != nil {
			t.Fatal(err)
		}
	}
	if data, err := os.ReadFile(filepath.Join(repo, m2EditedAsset)); err != nil || string(data) != "ci: deadcode\n\nscan:\n\tgolaw scan .\n" {
		t.Fatalf("golaw.mk = %q, %v", data, err)
	}
	for _, absent := range []string{m3Makefile, m3SecondRecord} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(absent))); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("the red shape wrote %s: %v", absent, err)
		}
	}
	if data, err := os.ReadFile(filepath.Join(repo, moduleFile)); err != nil || !strings.HasSuffix(string(data), "require (\n\texample.test/second v1.0.0\n)\n") {
		t.Fatalf("go.mod = %q, %v", data, err)
	}
}

// m3CaseScript adopts the service and answers every scan with the red or
// the clean document of the case's check, as the side's directory asks.
func m3CaseScript(t *testing.T, s *suite, red, near string) func(context.Context, command) result {
	t.Helper()
	var calls [][]string
	service := m3ServiceAnswer(t, s, &calls)
	return newM2Script(t, s, func(ctx context.Context, c command) (m2Reply, bool) {
		if c.program == s.binary && len(c.args) > 0 && c.args[0] == commandScan {
			if filepath.Base(c.dir) == "red" {
				return m2Reply{got: result{code: 1, stdout: red}, target: true}, true
			}
			return m2Reply{got: result{stdout: near}, target: true}, true
		}
		return service(ctx, c)
	}).Execute
}

func TestM3CaseRowNamesTheSideThatFailed(t *testing.T) {
	t.Parallel()
	clean := `{"schema":1,"checks":[{"id":"policy.makefile","status":"measured"}]}`
	found := `{"schema":1,"findings":[{"check":"policy.makefile","severity":"deny"}],"checks":[{"id":"policy.makefile","status":"measured"}]}`
	for _, tc := range []struct{ name, red, near, side string }{
		{"red the red edit draws no finding", clean, clean, "the red side"},
		{"red the near edit draws a finding", found, found, "the near side"},
		{"red the near scan answers no document", found, "golaw: broken", "the near side"},
	} {
		s := m2TestSuite(t)
		s.execute = m3CaseScript(t, s, tc.red, tc.near)
		if err := s.caseM3(t.Context(), "missing-include"); err == nil || !strings.Contains(err.Error(), tc.side) {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
	s := m2TestSuite(t)
	s.execute = func(context.Context, command) result { return result{err: ErrAcceptance} }
	if err := s.caseM3(t.Context(), "missing-include"); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("red a case over a failed adoption = %v", err)
	}
	ran := false
	s.execute = func(context.Context, command) result {
		ran = true
		return result{}
	}
	if err := s.caseM3(t.Context(), "absent"); !errors.Is(err, ErrAcceptance) || ran {
		t.Fatalf("red an unknown case = %v, a command ran %v", err, ran)
	}
}

func TestM3CaseSideStopsBeforeTheScan(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	scanned := false
	s.execute = func(context.Context, command) result {
		scanned = true
		return result{}
	}
	refuse := func(string) error { return ErrAcceptance }
	if err := s.m3CaseSide(t.Context(), filepath.Join(t.TempDir(), "absent"), "red", "policy.makefile", refuse); err == nil {
		t.Fatal("red a copy of an absent repository must fail")
	}
	repo := t.TempDir()
	checkedWrite(t, filepath.Join(repo, moduleFile), "module example.test/service\n")
	if err := s.m3CaseSide(t.Context(), repo, "near", "policy.makefile", refuse); !errors.Is(err, ErrAcceptance) || scanned {
		t.Fatalf("red a refused edit = %v, scanned %v", err, scanned)
	}
}

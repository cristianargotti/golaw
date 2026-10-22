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

func TestM3EditorsChangeTheTreeOrNameWhyNot(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := m3DisableShadow(repo); err == nil {
		t.Fatal("red a repository without a lint configuration must fail")
	}
	checkedWrite(t, filepath.Join(repo, moduleFile), "module example.test/service\n")
	checkedWrite(t, filepath.Join(repo, ".squawk.toml"), "excluded_rules = []\n")
	if err := m3Append(moduleFile, "\nrequire example.test/unrecorded v1.0.0\n")(repo); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(repo, moduleFile)); err != nil || !strings.HasSuffix(string(data), "unrecorded v1.0.0\n") {
		t.Fatalf("go.mod = %q, %v", data, err)
	}
	if err := m3Remove(".squawk.toml")(repo); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".squawk.toml")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the removed file = %v", err)
	}
	if err := m3Append("absent.toml", "x = 1\n")(repo); err == nil {
		t.Fatal("red an append to an absent file must fail")
	}
	for _, body := range []string{"", "linters: [\n", "run:\n  timeout: 5m\n", "linters:\n  settings:\n    govet:\n      enable-all: true\n"} {
		checkedWrite(t, filepath.Join(repo, ".golangci.yml"), body)
		if err := m3DisableShadow(repo); err == nil {
			t.Fatalf("red lint configuration %q was edited", body)
		}
	}
	checkedWrite(t, filepath.Join(repo, "Dockerfile"), "FROM golang:1.27 AS build\n")
	if err := m3DropDigest(repo); err == nil {
		t.Fatal("red a Dockerfile without a digest must fail")
	}
}

// m3RowScript answers the adoption of the service, the scan with a given
// document and make with a given result, and records whether make ran.
func m3RowScript(t *testing.T, s *suite, doc string, made result, ran *bool) func(context.Context, command) result {
	t.Helper()
	var calls [][]string
	service := m3ServiceAnswer(t, s, &calls)
	return newM2Script(t, s, func(ctx context.Context, c command) (m2Reply, bool) {
		switch {
		case c.program == s.binary && len(c.args) > 0 && c.args[0] == commandScan:
			return m2Reply{got: result{stdout: doc}}, true
		case c.program == s.m2.Make:
			*ran = true
			return m2Reply{got: made, target: true}, true
		}
		return service(ctx, c)
	}).Execute
}

func TestM3ViolationRowStopsBeforeAnUnprovenEdit(t *testing.T) {
	t.Parallel()
	violation := m3Violation{check: "policy.makefile", capabilities: []string{"go"}, apply: m3Remove("absent.yaml")}
	for _, tc := range []struct{ name, doc string }{
		{"red the check is not clean before the edit", `{"schema":1,"checks":[{"id":"policy.makefile","status":"unmeasured"}]}`},
		{"red the edit finds nothing", `{"schema":1,"checks":[{"id":"policy.makefile","status":"measured"}]}`},
		{"red the scan answers no document", "golaw: broken"},
	} {
		s := m2TestSuite(t)
		s.m2.Make = "/simulated/make"
		ran := false
		s.execute = m3RowScript(t, s, tc.doc, result{code: 2, stdout: "policy.makefile\n"}, &ran)
		if err := s.m3ViolationRow(t.Context(), "makefile", violation); err == nil || ran {
			t.Fatalf("%s: %v, make ran %v", tc.name, err, ran)
		}
	}
}

func TestM3PolicyRowsStopWhenTheServiceCannotBeAdopted(t *testing.T) {
	t.Parallel()
	for _, name := range []string{m3Baseline, m3ProfileChecks, "makefile"} {
		s := m2TestSuite(t)
		s.execute = func(context.Context, command) result { return result{err: ErrAcceptance} }
		if err := s.policyM3(t.Context(), name); !errors.Is(err, ErrAcceptance) {
			t.Fatalf("%s over a failed adoption = %v", name, err)
		}
	}
	s := m2TestSuite(t)
	ran := false
	s.execute = m3RowScript(t, s, "golaw: broken", result{}, &ran)
	if err := s.policyM3(t.Context(), m3ProfileChecks); err == nil || ran {
		t.Fatalf("red the profile row over an unreadable scan = %v, make ran %v", err, ran)
	}
}

func TestM3PassesCIRequiresAStartedMakeAndAnUnchangedTree(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		got  result
		edit bool
	}{
		{"red make not started", result{err: ErrAcceptance}, false},
		{"red make rewrote a tracked file", result{}, true},
	} {
		s := m2TestSuite(t)
		s.m2.Make = "/simulated/make"
		var calls [][]string
		service := m3ServiceAnswer(t, s, &calls)
		s.execute = newM2Script(t, s, func(ctx context.Context, c command) (m2Reply, bool) {
			if c.program == s.m2.Make {
				if tc.edit {
					checkedWrite(t, filepath.Join(c.dir, m2EditedAsset), "rewritten by a target\n")
				}
				return m2Reply{got: tc.got, target: true}, true
			}
			return service(ctx, c)
		}).Execute
		if err := s.policyM3(t.Context(), m3Baseline); err == nil {
			t.Fatalf("%s: the baseline passed", tc.name)
		}
	}
}

func TestM3ScanNamesAnUnreadableDocument(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	for _, got := range []result{{err: ErrAcceptance}, {code: 2, stdout: "not a document", stderr: "golaw: broken\n"}} {
		s.execute = func(context.Context, command) result { return got }
		if _, _, err := s.m3Scan(t.Context(), t.TempDir()); err == nil {
			t.Fatalf("red scan %+v decoded", got)
		}
	}
}

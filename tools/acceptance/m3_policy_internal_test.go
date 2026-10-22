package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	scanreport "github.com/cristianargotti/golaw/internal/report"
)

func TestM3PolicyNamesOneRowPerCheckAndTheProfileRow(t *testing.T) {
	t.Parallel()
	names := m3PolicyNames()
	for name, violation := range m3Violations() {
		if !slices.Contains(names, name) || !strings.HasPrefix(violation.check, "policy.") || !slices.Contains(violation.capabilities, "go") {
			t.Fatalf("%s: %+v", name, violation)
		}
	}
	if len(names) != len(m3Violations())+2 || !slices.Contains(names, m3Baseline) || !slices.Contains(names, m3ProfileChecks) {
		t.Fatalf("names = %q", names)
	}
}

func TestM3JudgesOfAViolation(t *testing.T) {
	t.Parallel()
	clean := scanreport.Document{Checks: []scanreport.Status{{ID: "policy.makefile", Status: "measured"}}}
	if err := m3JudgeClean(clean, "policy.makefile"); err != nil {
		t.Fatalf("near clean = %v", err)
	}
	found := clean
	found.Findings = []model.Finding{{Check: "policy.makefile", Severity: model.SeverityDeny}}
	for _, doc := range []scanreport.Document{found, {Checks: []scanreport.Status{{ID: "policy.makefile", Status: "unmeasured"}}}, {}} {
		if err := m3JudgeClean(doc, "policy.makefile"); err == nil {
			t.Fatalf("red clean judge passed %+v", doc)
		}
	}
	for _, tc := range []struct {
		name string
		got  result
		fail bool
	}{
		{"near the scan target names the check", result{code: 2, stdout: "Makefile:3:1 deny policy.makefile ...\nmake: *** [scan] Error 1\n"}, false},
		{"red make ci passed", result{code: 0, stdout: "policy.makefile\n"}, true},
		{"red another target failed", result{code: 2, stdout: "make: *** [lint] Error 1\n"}, true},
		{"red not started", result{err: ErrAcceptance}, true},
	} {
		if err := m3JudgeViolation("policy.makefile", tc.got); (err != nil) != tc.fail {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
}

func TestM3FileEditorsRefuseWhatTheyCannotFind(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	checkedWrite(t, filepath.Join(repo, "golaw.mk"), "export GOTOOLCHAIN=local\n\nci: mod deadcode scan\n\ndeadcode:\n\tgo test ./test/deadcode/\n\nscan:\n\tgolaw scan .\n")
	for _, edit := range []func(string) error{
		m3RemovePrerequisite("golaw.mk", "ci", "deadcode"), m3RemoveRule("golaw.mk", "deadcode"),
		m3Replace("golaw.mk", "export GOTOOLCHAIN=local\n", ""),
	} {
		if err := edit(repo); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(filepath.Join(repo, "golaw.mk"))
	if err != nil || string(got) != "\nci: mod scan\n\nscan:\n\tgolaw scan .\n" {
		t.Fatalf("edited golaw.mk = %q, %v", got, err)
	}
	for _, edit := range []func(string) error{
		m3RemovePrerequisite("golaw.mk", "ci", "deadcode"), m3RemoveRule("golaw.mk", "deadcode"),
		m3Replace("golaw.mk", "-days 7", "-days 1"), m3Remove("absent.yaml"),
	} {
		if err := edit(repo); err == nil {
			t.Fatal("an edit of something absent must fail, or the row would judge an unchanged tree")
		}
	}
	checkedWrite(t, filepath.Join(repo, ".golangci.yml"), "linters:\n  settings:\n    govet:\n      enable-all: true\n      disable: [fieldalignment]\n")
	if err := m3DisableShadow(repo); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(repo, ".golangci.yml")); err != nil || !strings.Contains(string(data), "shadow") {
		t.Fatalf("govet = %q, %v", data, err)
	}
	checkedWrite(t, filepath.Join(repo, "Dockerfile"), "FROM golang:1.27@sha256:"+strings.Repeat("a", 64)+" AS build\n")
	if err := m3DropDigest(repo); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(repo, "Dockerfile")); err != nil || string(data) != "FROM golang:1.27 AS build\n" {
		t.Fatalf("Dockerfile = %q, %v", data, err)
	}
}

func TestM3PolicyRowFailsTheCIOverItsViolation(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	s.m2.Make = "/simulated/make"
	var calls [][]string
	service := m3ServiceAnswer(t, s, &calls)
	var made []string
	s.execute = newM2Script(t, s, func(ctx context.Context, c command) (m2Reply, bool) {
		switch {
		case c.program == s.binary && len(c.args) > 0 && c.args[0] == commandScan:
			return m2Reply{got: result{stdout: `{"schema":1,"checks":[{"id":"policy.deps-age-config","status":"measured"}]}`}}, true
		case c.program == s.m2.Make:
			made = append(made, strings.Join(c.args, " "))
			return m2Reply{got: result{code: 2, stdout: "golaw.mk:1:1 deny policy.deps-age-config ...\n"}, target: true}, true
		}
		return service(ctx, c)
	}).Execute
	if err := s.policyM3(t.Context(), "deps-age-config"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(made, []string{"-k ci"}) {
		t.Fatalf("make calls = %q", made)
	}
}

func TestM3ProfileChecksWaitForAProfile(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, status string
		fail         bool
	}{
		{"near not applicable without a profile", "not-applicable", false},
		{"red applied without a profile", "measured", true},
	} {
		s := m2TestSuite(t)
		var calls [][]string
		service := m3ServiceAnswer(t, s, &calls)
		reason := `"reason":"no profile is enrolled; the profile turns this rule on"`
		doc := `{"schema":1,"checks":[{"id":"policy.hook-tests","status":"` + tc.status + `",` + reason + `},` +
			`{"id":"policy.codeowners-foreign","status":"` + tc.status + `",` + reason + `}]}`
		s.execute = newM2Script(t, s, func(ctx context.Context, c command) (m2Reply, bool) {
			if c.program == s.binary && len(c.args) > 0 && c.args[0] == commandScan {
				return m2Reply{got: result{stdout: doc}, target: true}, true
			}
			return service(ctx, c)
		}).Execute
		if err := s.policyM3(t.Context(), m3ProfileChecks); (err != nil) != tc.fail {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
}

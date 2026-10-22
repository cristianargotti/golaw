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

func TestM3CapabilityViolationsReachOnlyTheTreeThatHoldsTheirFile(t *testing.T) {
	t.Parallel()
	with, without := t.TempDir(), t.TempDir()
	lint := "linters:\n  settings:\n    depguard:\n      rules:\n        domain:\n          list-mode: strict\n        main:\n          list-mode: lax\n"
	checkedWrite(t, filepath.Join(with, m3LintFile), lint)
	checkedWrite(t, filepath.Join(without, m3LintFile), "linters:\n  settings:\n    depguard:\n      rules:\n        main:\n          list-mode: lax\n")
	checkedWrite(t, filepath.Join(with, ".squawk.toml"), "excluded_rules = []\n")
	for _, repo := range []string{with, without} {
		for _, name := range []string{"api", "db", "hexagonal"} {
			if err := m3CapabilityCases()[name].apply(repo); err != nil {
				t.Fatalf("%s over %s: %v", name, repo, err)
			}
		}
		requireCapabilityViolations(t, repo)
	}
}

// requireCapabilityViolations requires the state the api, db and hexagonal
// violations leave in a tree, whichever of their files it held before.
func requireCapabilityViolations(t *testing.T, repo string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo, m3LintFile))
	if err != nil || strings.Contains(string(data), "domain:") || !strings.Contains(string(data), "main:") {
		t.Fatalf("the depguard rules = %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".squawk.toml")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the squawk configuration = %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(m3RawResponseFile))); err != nil || !strings.Contains(string(data), "r.JSON(500,") {
		t.Fatalf("the raw response = %q, %v", data, err)
	}
}

func TestM3CapabilityViolationsRefuseWhatTheyCannotEdit(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	for _, edit := range []func(string) error{m3DropDomainRule, m3DropDigests} {
		if err := edit(repo); err == nil {
			t.Fatal("red an edit of an absent file must fail")
		}
	}
	checkedWrite(t, filepath.Join(repo, m3LintFile), "linters:\n  settings:\n    depguard: {}\n")
	if err := m3DropDomainRule(repo); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("red a configuration without depguard rules = %v", err)
	}
	checkedWrite(t, filepath.Join(repo, "Dockerfile"), "FROM golang:1.27@sha256:"+strings.Repeat("a", 64)+" AS build\n"+
		"FROM scratch@sha256:"+strings.Repeat("b", 64)+"\n")
	if err := m3DropDigests(repo); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(repo, "Dockerfile")); err != nil || string(data) != "FROM golang:1.27 AS build\nFROM scratch\n" {
		t.Fatalf("Dockerfile = %q, %v", data, err)
	}
	if err := m3DropDigests(repo); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("red a Dockerfile without a digest = %v", err)
	}
	if err := m3RemovePresent("absent.toml")(repo); err != nil {
		t.Fatalf("near an absent file = %v", err)
	}
	checkedWrite(t, filepath.Join(repo, "full", "file.txt"), "kept\n")
	if err := m3RemovePresent("full")(repo); err == nil {
		t.Fatal("red a directory that holds a file must not be removed")
	}
	checkedWrite(t, filepath.Join(repo, "internal", "server"), "a file where the package directory goes\n")
	if err := m3WriteRawResponse(repo); err == nil {
		t.Fatal("red a raw response under a file must fail")
	}
}

func TestM3JudgeCapabilityNamesTheCapability(t *testing.T) {
	t.Parallel()
	api := m3CapabilityCases()["api"]
	with := capabilityDocument("goast.raw-error-response", "measured", true)
	without := capabilityDocument("goast.raw-error-response", "not-applicable", false)
	if err := m3JudgeCapability("api", api, with, without); err == nil {
		t.Fatal("red a reason that names another capability must fail")
	}
	hexagonal := m3CapabilityCases()["hexagonal"]
	if err := m3JudgeCapability("hexagonal", hexagonal, capabilityDocument("policy.golangci", "measured", true),
		capabilityDocument("policy.golangci", "unmeasured", false)); err == nil {
		t.Fatal("red a shaped rule left unmeasured without the capability must fail")
	}
}

// m3CapabilityScript adopts the service and answers each scan with the
// document the tree's directory and its Dockerfile ask for.
func m3CapabilityScript(t *testing.T, s *suite, answer func(dir string) result) func(context.Context, command) result {
	t.Helper()
	var calls [][]string
	service := m3ServiceAnswer(t, s, &calls)
	return newM2Script(t, s, func(ctx context.Context, c command) (m2Reply, bool) {
		if c.program == s.binary && len(c.args) > 0 && c.args[0] == commandScan {
			return m2Reply{got: answer(c.dir), target: true}, true
		}
		return service(ctx, c)
	}).Execute
}

func TestM3CapabilityRowStopsWhereItsProofBreaks(t *testing.T) {
	t.Parallel()
	clean := `{"schema":1,"checks":[{"id":"policy.image-digest","status":"measured"}]}`
	found := `{"schema":1,"findings":[{"check":"policy.image-digest","severity":"deny"}],"checks":[{"id":"policy.image-digest","status":"measured"}]}`
	for _, tc := range []struct {
		name   string
		answer func(dir string) result
	}{
		{"red a finding before the violation", func(string) result { return result{code: 1, stdout: found} }},
		{"red the first scan answers no document", func(string) result { return result{code: 2, stdout: "golaw: broken"} }},
		{"red a later scan answers no document", func(dir string) result {
			if data, err := os.ReadFile(filepath.Join(dir, "Dockerfile")); err == nil && strings.Contains(string(data), "@sha256:") {
				return result{stdout: clean}
			}
			return result{code: 2, stdout: "golaw: broken"}
		}},
	} {
		s := m2TestSuite(t)
		s.execute = m3CapabilityScript(t, s, tc.answer)
		if err := s.capabilityM3(t.Context(), "image"); err == nil {
			t.Fatalf("%s: the row passed", tc.name)
		}
	}
	s := m2TestSuite(t)
	s.execute = func(context.Context, command) result { return result{err: ErrAcceptance} }
	if err := s.capabilityM3(t.Context(), "db"); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("red a row over a failed adoption = %v", err)
	}
}

func TestM3CapabilityRowStopsWhenTheSecondTreeFails(t *testing.T) {
	t.Parallel()
	clean := `{"schema":1,"checks":[{"id":"policy.golangci","status":"measured"}]}`
	s := m2TestSuite(t)
	script := m3CapabilityScript(t, s, func(string) result { return result{stdout: clean} })
	s.execute = func(ctx context.Context, c command) result {
		if c.program == s.binary && len(c.args) > 0 && c.args[0] == commandInit && filepath.Base(c.dir) == "without" {
			return result{code: 2, stderr: "golaw: refused\n"}
		}
		return script(ctx, c)
	}
	if err := s.capabilityM3(t.Context(), "hexagonal"); err == nil {
		t.Fatal("red a row whose second tree cannot be adopted must fail")
	}
	s = m2TestSuite(t)
	s.execute = m3CapabilityScript(t, s, func(string) result { return result{stdout: clean} })
	checkedWrite(t, filepath.Join(s.base, dirRepos, "without", m3LintFile), "linters: [errcheck]\n")
	if err := s.capabilityM3(t.Context(), "hexagonal"); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("red a violation that cannot edit the second tree = %v", err)
	}
}

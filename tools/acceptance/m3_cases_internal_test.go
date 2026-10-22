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

func TestM3CaseNamesAreTheSixOfTheMilestone(t *testing.T) {
	t.Parallel()
	want := []string{"disabled-linter", "generated-file-allowlist", "hollowed-by-exclusion", "missing-include", "modular-makefile", "second-require-block"}
	if !slices.Equal(m3CaseNames(), want) {
		t.Fatalf("cases = %q", m3CaseNames())
	}
}

func TestM3JudgeCase(t *testing.T) {
	t.Parallel()
	measured := scanreport.Document{Checks: []scanreport.Status{{ID: "policy.golangci", Status: "measured"}}}
	found := measured
	found.Findings = []model.Finding{{Check: "policy.golangci", Severity: model.SeverityDeny}}
	for _, tc := range []struct {
		name string
		red  bool
		doc  scanreport.Document
		code int
		fail bool
	}{
		{"red found", true, found, 1, false},
		{"red missed", true, measured, 0, true},
		{"near clean", false, measured, 0, false},
		{"near over-fired", false, found, 1, true},
		{"near unmeasured", false, scanreport.Document{Checks: []scanreport.Status{{ID: "policy.golangci", Status: "unmeasured"}}}, 2, true},
	} {
		if err := m3JudgeCase("policy.golangci", tc.red, tc.doc, tc.code); (err != nil) != tc.fail {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
}

func TestM3CaseEditorsReproduceEachShape(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	checkedWrite(t, filepath.Join(repo, ".golangci.yml"), "linters:\n  default: standard\n  exclusions:\n    rules: []\n")
	checkedWrite(t, filepath.Join(repo, "golaw.mk"), "ci: deadcode\n\ndeadcode:\n\tgo test ./test/deadcode/\n")
	checkedWrite(t, filepath.Join(repo, "Makefile"), "include golaw.mk\n")
	checkedWrite(t, filepath.Join(repo, "go.mod"), "module example.test/service\n\ngo 1.27\n")
	for _, edit := range []func(string) error{
		m3Disable("errcheck"), m3Exclude("internal/", "errcheck"), m3MoveRule("deadcode", true), m3SecondBlock(true),
	} {
		if err := edit(repo); err != nil {
			t.Fatal(err)
		}
	}
	for path, fragments := range map[string][]string{
		".golangci.yml":                  {"disable:", "errcheck", "path: internal/"},
		"golaw.mk":                       {"ci: deadcode"},
		"mk/quality.mk":                  {"deadcode:\n\tgo test ./test/deadcode/\n"},
		"Makefile":                       {"include golaw.mk\ninclude mk/quality.mk\n"},
		"go.mod":                         {"require (\n\texample.test/second v1.0.0\n)\n"},
		"docs/adr/0002-second-module.md": {"modules:\n  - example.test/second\n"},
	} {
		data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		for _, fragment := range fragments {
			if !strings.Contains(string(data), fragment) {
				t.Fatalf("%s lacks %q:\n%s", path, fragment, data)
			}
		}
	}
	if data, err := os.ReadFile(filepath.Join(repo, "golaw.mk")); err != nil || strings.Contains(string(data), "deadcode:\n") {
		t.Fatalf("the moved rule stayed in golaw.mk: %q, %v", data, err)
	}
}

func TestM3CaseRowScansTheRedAndTheNearTree(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	var calls [][]string
	service := m3ServiceAnswer(t, s, &calls)
	scanned := map[string]bool{}
	s.execute = newM2Script(t, s, func(ctx context.Context, c command) (m2Reply, bool) {
		if c.program == s.binary && len(c.args) > 0 && c.args[0] == commandScan {
			data, err := os.ReadFile(filepath.Join(c.dir, ".gitleaks.toml"))
			red := err == nil && strings.Contains(string(data), "paths = ")
			scanned[filepath.Base(c.dir)] = true
			doc := `{"schema":1,"checks":[{"id":"policy.gitleaks-generic","status":"measured"}]}`
			if red {
				return m2Reply{got: result{code: 1, stdout: `{"schema":1,"findings":[{"check":"policy.gitleaks-generic","severity":"deny"}],` +
					`"checks":[{"id":"policy.gitleaks-generic","status":"measured"}]}`}, target: true}, true
			}
			return m2Reply{got: result{stdout: doc}, target: true}, true
		}
		return service(ctx, c)
	}).Execute
	if err := s.caseM3(t.Context(), "generated-file-allowlist"); err != nil {
		t.Fatal(err)
	}
	if !scanned["red"] || !scanned["near"] {
		t.Fatalf("scanned = %v", scanned)
	}
}

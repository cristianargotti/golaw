package cicheck_test

import (
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const kitModule = "github.com/cristianargotti/golaw/internal/"

// depguardRule is one rule of the package direction in .golangci.yml.
type depguardRule struct {
	ListMode string   `yaml:"list-mode"`
	Files    []string `yaml:"files"`
	Allow    []string `yaml:"allow"`
}

// lintConfig is the part of .golangci.yml that draws the package direction.
type lintConfig struct {
	Linters struct {
		Settings struct {
			Depguard struct {
				Rules map[string]depguardRule `yaml:"rules"`
			} `yaml:"depguard"`
		} `yaml:"settings"`
	} `yaml:"linters"`
}

// TestDepguardDrawsTheDirectionOfEveryNewPackage holds every package the
// rungs, setup and the staged scan bring to a strict list of what it may
// import, so a package that reaches past its layer fails the lint.
func TestDepguardDrawsTheDirectionOfEveryNewPackage(t *testing.T) {
	t.Parallel()
	var config lintConfig
	if err := yaml.Unmarshal(kitBytes(t, ".golangci.yml"), &config); err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"runner":     {"model"},
		"coverage":   {"model"},
		"vuln":       {"model"},
		"stage":      {"model", "engine", "source", "policy", "report"},
		"rung":       {"model", "engine", "policy", "source", "stage", "runner", "report", "vuln"},
		"setup":      {"model", "runner", "install"},
		"gittest":    nil,
		"runnertest": nil,
		// The profile bundle and the release verification of M5: the loader
		// reads YAML beside model and policy, its test support reaches the
		// loader and gittest, and the verifier is standard cryptography alone.
		"profile":     {"model", "policy", "runner", "gopkg.in/yaml.v3"},
		"profiletest": {"model", "policy", "profile", "gittest"},
		"release":     nil,
	}
	for name, imports := range want {
		rule, ok := config.Linters.Settings.Depguard.Rules[name]
		if !ok {
			t.Errorf("depguard has no rule %s", name)
			continue
		}
		if rule.ListMode != "strict" || !slices.Equal(rule.Files, []string{"!$test", "**/internal/" + name + "/**"}) {
			t.Errorf("rule %s = %+v, want strict over internal/%s outside tests", name, rule, name)
		}
		allow := make([]string, 0, 1+len(imports))
		allow = append(allow, "$gostd")
		for _, imported := range imports {
			// A third-party module is named whole; a kit package by its name.
			if strings.Contains(imported, ".") {
				allow = append(allow, imported)
				continue
			}
			allow = append(allow, kitModule+imported)
		}
		if !slices.Equal(rule.Allow, allow) {
			t.Errorf("rule %s allows %v, want %v", name, rule.Allow, allow)
		}
	}
}

// lintExclusions is the part of .golangci.yml that frees a path from a linter.
type lintExclusions struct {
	Linters struct {
		Exclusions struct {
			Rules []struct {
				Path    string   `yaml:"path"`
				Text    string   `yaml:"text"`
				Linters []string `yaml:"linters"`
			} `yaml:"rules"`
		} `yaml:"exclusions"`
	} `yaml:"linters"`
}

// TestTestFilesLoseTheFunctionLimit holds the law of section 3 as the lint
// applies it: a test file keeps the file limit and loses the function
// limit, which revive's function-length holds beside funlen.
func TestTestFilesLoseTheFunctionLimit(t *testing.T) {
	t.Parallel()
	var config lintExclusions
	if err := yaml.Unmarshal(kitBytes(t, ".golangci.yml"), &config); err != nil {
		t.Fatal(err)
	}
	for _, rule := range config.Linters.Exclusions.Rules {
		if rule.Path == `_test\.go` && rule.Text == "function-length" && slices.Equal(rule.Linters, []string{"revive"}) {
			return
		}
	}
	t.Error(".golangci.yml holds the functions of a test file to 40 lines through revive")
}

// TestMakefileLintsOnePackage keeps the target a task runs to lint the one
// package it changed from the module root, so no command line needs a cd.
func TestMakefileLintsOnePackage(t *testing.T) {
	t.Parallel()
	requireText(t, "Makefile", "lint-pkg:\n\tgolangci-lint run $(PKG)\n", ".PHONY: mod deps-age fmt lint lint-pkg ")
}

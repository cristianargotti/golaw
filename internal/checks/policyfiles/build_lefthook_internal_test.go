package policyfiles

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestLefthookReaderKeepsCommandsAndScripts(t *testing.T) {
	t.Parallel()
	const file = "lefthook.yml"
	for _, tc := range []struct {
		name, body string
		count      int
		limited    bool
	}{
		{"near absent entries", "min_version: 2.1.0\n", 0, false},
		{"near command", "pre-commit:\n  commands:\n    check:\n      run: golaw hook pre-commit\n", 1, false},
		{"near script", "pre-commit:\n  scripts:\n    check.sh:\n      runner: sh\n", 1, false},
		{"near merged command", "base: &base\n  run: golaw hook pre-commit\npre-commit:\n  commands:\n    check:\n      <<: *base\n", 1, false},
		{"red computed command", "pre-commit:\n  commands:\n    check:\n      run: [golaw, hook, pre-commit]\n", 0, true},
		{"red external config", "extends: [other.yml]\n", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := readLefthook(file, []byte(tc.body))
			if tc.limited {
				if !errors.Is(err, model.ErrOutsideSubset) {
					t.Fatalf("limit = %v", err)
				}
				return
			}
			if err != nil || len(got) != tc.count {
				t.Fatalf("commands = %+v, %v", got, err)
			}
			requireLefthookCommandSource(t, got, tc.count)
		})
	}
}

func requireLefthookCommandSource(t *testing.T, got []lefthookCommand, count int) {
	t.Helper()
	if count > 0 && (got[0].hook != "pre-commit" || got[0].line == 0) {
		t.Fatalf("command source = %+v", got[0])
	}
}

func TestLefthookReaderRejectsMalformedYAML(t *testing.T) {
	t.Parallel()
	_, err := readLefthook("lefthook.yml", []byte("pre-commit: ["))
	if err == nil {
		t.Fatal("malformed YAML was accepted")
	}
}

func TestLefthookReaderPreservesEveryHookAndGroupOrder(t *testing.T) {
	t.Parallel()
	const body = `base: &command
  run: golaw hook pre-commit && echo ready
metadata:
  description: ignored
post-checkout:
  scripts:
    prepare.sh:
      runner: sh
  commands:
    refresh: *command
pre-commit:
  commands:
    disabled:
      skip: true
      run: make ci
  scripts:
    check.sh:
      run: not a direct command
`
	got, err := readLefthook("lefthook.yml", []byte(body))
	want := []lefthookCommand{
		{hook: "post-checkout", name: "prepare.sh", line: 7},
		{hook: "post-checkout", name: "refresh", run: "golaw hook pre-commit && echo ready", line: 10},
		{hook: "pre-commit", name: "disabled", run: "make ci", line: 13},
		{hook: "pre-commit", name: "check.sh", line: 17},
	}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("commands = %+v, %v, want %+v", got, err, want)
	}
}

func TestLefthookReaderPreservesMergedOverrides(t *testing.T) {
	t.Parallel()
	const body = "base: &base {run: ignored, skip: true}\n" +
		"post-merge:\n  commands:\n    check:\n      <<: *base\n      run: echo ready\n"
	got, err := readLefthook("lefthook.yml", []byte(body))
	want := []lefthookCommand{{hook: "post-merge", name: "check", run: "echo ready", line: 4}}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("commands = %+v, %v, want %+v", got, err, want)
	}
}

func TestLefthookReaderNamesUnsupportedRelevantForms(t *testing.T) {
	t.Parallel()
	const file = "lefthook.yml"
	for _, tc := range []struct {
		name, body, location, construct string
	}{
		{"remotes", "remotes:\n  - git_url: https://example.test/hooks.git\n", ":1:", "remotes"},
		{"jobs sequence", "post-checkout:\n  jobs:\n    - run: make ci\n", ":2:", "jobs"},
		{"commands sequence", "pre-commit:\n  commands: []\n", ":2:", "commands"},
		{"scripts sequence", "pre-commit:\n  scripts: []\n", ":2:", "scripts"},
		{"scalar command", "pre-commit:\n  commands:\n    check: make ci\n", ":3:", "check"},
		{"run mapping", "pre-commit:\n  commands:\n    check:\n      run: {program: make}\n", ":4:", "run"},
		{"duplicate command run", "pre-commit:\n  commands:\n    check:\n      run: make ci\n      run: echo ready\n", ":5:", "duplicate"},
		{"scalar external source", "extends: other.yml\n", ":1:", "extends"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := readLefthook(file, []byte(tc.body))
			if !errors.Is(err, model.ErrOutsideSubset) || !strings.Contains(err.Error(), file+tc.location) ||
				!strings.Contains(err.Error(), tc.construct) {
				t.Fatalf("limit = %v, want %s%s and %q", err, file, tc.location, tc.construct)
			}
		})
	}
}

func TestLefthookReaderNamesTheSuppliedPath(t *testing.T) {
	t.Parallel()
	const file = "config/hooks.yml"
	const body = "pre-commit:\n  commands:\n    check:\n      run: [make, ci]\n"
	_, err := readLefthook(file, []byte(body))
	if !errors.Is(err, model.ErrOutsideSubset) || !strings.Contains(err.Error(), file+":4:") ||
		!strings.Contains(err.Error(), "run") {
		t.Fatalf("nested source = %v", err)
	}
}

func TestLefthookReaderIgnoresEmptyExternalSources(t *testing.T) {
	t.Parallel()
	for _, body := range []string{"extends: []\nremotes: []\n", "extends: null\nremotes: ''\n", "extends: {}\n"} {
		got, err := readLefthook("lefthook.yml", []byte(body))
		if err != nil || len(got) != 0 {
			t.Fatalf("empty external source %q = %+v, %v", body, got, err)
		}
	}
}

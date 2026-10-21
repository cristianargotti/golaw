package doctor

import (
	"strings"
	"testing"
)

func TestAssessGoNamesTheGapAndTheCure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		facts  GoFacts
		state  string
		code   string
		detail string
	}{
		{
			"red older than the go line",
			GoFacts{Local: "go1.27.0", Line: "1.27.2", Tools: "1.27.2"},
			"older", "go-older-than-line",
			"local go1.27.0 is older than the go line 1.27.2 of go.mod, and golaw.mk exports GOTOOLCHAIN=local, so every Go target stops: install go1.27.2",
		},
		{
			"red older than the tools module",
			GoFacts{Local: "go1.26.0", Line: "1.24", Tools: "1.26.6"},
			"older", "go-older-than-tools",
			"local go1.26.0 is older than the go line 1.26.6 of tools/go.mod, which golaw setup builds the pinned tools with: install go1.26.6 or newer before golaw setup",
		},
		{"red unreadable", GoFacts{Error: "go env GOVERSION: exit 1"}, "unknown", "go-unreadable", "go env GOVERSION: exit 1"},
		{"red no version", GoFacts{Local: "devel go1.28-abc", Line: "1.27"}, "unknown", "go-unreadable", "devel go1.28-abc"},
		{"near equal", GoFacts{Local: "go1.27.2", Line: "1.27.2", Tools: "1.27.2"}, "ok", "go-meets-line", "local go1.27.2 meets the go line 1.27.2 of go.mod"},
		{"near newer without a tools module", GoFacts{Local: "go1.27.3", Line: "1.25"}, "ok", "go-meets-line", "setup-go with go-version-file: go.mod"},
	} {
		lines := AssessGo(tc.facts)
		if len(lines) != 1 {
			t.Fatalf("%s: %d lines", tc.name, len(lines))
		}
		line := lines[0]
		if line.ID != idGoToolchain || line.Key != "go" || line.State != tc.state || line.Code != tc.code ||
			!strings.Contains(line.Detail, tc.detail) {
			t.Fatalf("%s: line = %+v", tc.name, line)
		}
	}
}

func TestAssessGoNamesInvalidModuleVersions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		facts GoFacts
		path  string
		raw   string
	}{
		{"invalid main line", GoFacts{Local: "go1.27.0", Line: "invalid"}, "go.mod", "invalid"},
		{"missing main line", GoFacts{Local: "go1.27.0"}, "go.mod", "\"\""},
		{"invalid tools line", GoFacts{Local: "go1.27.0", Line: "1.24", Tools: "invalid"}, "tools/go.mod", "invalid"},
	} {
		lines := AssessGo(tc.facts)
		if len(lines) != 1 || lines[0].State != stateUnknown || lines[0].Code != "go-unreadable" {
			t.Fatalf("%s: lines = %+v", tc.name, lines)
		}
		for _, text := range []string{tc.facts.Local, tc.path, tc.raw} {
			if !strings.Contains(lines[0].Detail, text) {
				t.Errorf("%s: detail = %q, want %q", tc.name, lines[0].Detail, text)
			}
		}
	}
}

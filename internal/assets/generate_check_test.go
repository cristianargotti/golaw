package assets_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
)

// recipe answers the command lines of one target of a rendered makefile.
func recipe(body, target string) []string {
	var out []string
	inside := false
	for line := range strings.SplitSeq(body, "\n") {
		switch {
		case strings.HasPrefix(line, target+":"):
			inside = true
		case inside && strings.HasPrefix(line, "\t"):
			out = append(out, strings.TrimPrefix(line, "\t"))
		case inside:
			return out
		}
	}
	return out
}

func TestGenerateCheckRunsTheGeneratorCheckFirst(t *testing.T) {
	t.Parallel()
	for _, caps := range [][]string{{"go"}, {"api", "db", "go", "hexagonal"}} {
		context := assetContext()
		context.Capabilities = caps
		body := string(renderedAsset(t, "golaw.mk", context).Data)
		want := []string{"golaw gen --check .", "go generate ./...", "git diff --exit-code"}
		if got := recipe(body, "generate-check"); !slices.Equal(got, want) {
			t.Errorf("generate-check for %v = %q, want %q", caps, got, want)
		}
		if !slices.Contains(policyfiles.RequiredTargets(caps), "generate-check") {
			t.Errorf("generate-check left the required targets of %v", caps)
		}
	}
}

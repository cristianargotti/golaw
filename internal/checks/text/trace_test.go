package text_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/text"
)

// The forbidden words are assembled from halves so this file carries none.
func TestTraceTerms(t *testing.T) {
	t.Parallel()
	res := run(t, text.TraceTerms(), map[string]string{
		"docs/a.md": "Co-Auth" + "ored-By: x\nthe scr" + "atchpad dir\nsub-" + "agents run\nsee /Us" + "ers/me/x\n",
		"b.yml":     "paths: .github/work" + "flows/ci.yml\nusers/profile\nGener" + "ated With care\n",
	})
	// Targets come in path order, so b.yml is read before docs/a.md.
	want := []string{"trailer", "trace-term", "trace-term", "home-path"}
	if got := codes(res); !slices.Equal(got, want) {
		t.Errorf("codes = %v, want %v", got, want)
	}
	if res.Findings[0].Path != "docs/a.md" || res.Findings[0].Line != 1 {
		t.Errorf("trailer finding = %+v", res.Findings[0])
	}
	if res.Findings[3].Path != "docs/a.md" || res.Findings[3].Line != 4 || res.Findings[3].Column != 5 {
		t.Errorf("home path finding = %+v", res.Findings[3])
	}
}

// TestTraceTermsReadAdopterProse keeps the core rule to the traces it
// names: the pipeline word and the phrase of a generator are ordinary
// prose in an adopting repository, and a trailer phrase counts only in
// trailer form, at the start of a line or as the marker of a generated
// commit.
func TestTraceTermsReadAdopterProse(t *testing.T) {
	t.Parallel()
	res := run(t, text.TraceTerms(), map[string]string{
		"README.md": "The CI work" + "flow runs make ci.\nThe mocks are gener" + "ated with mockery.\n" +
			"a note on sign" + "ed-off-by: lines\n",
		".github/workflows/go.yml": "# This work" + "flow will build a golang project\n",
		"docs/commit.txt":          "  Sign" + "ed-off-by: A <a@example.invalid>\nGener" + "ated with [a tool](https://example.invalid)\n",
	})
	want := []string{"trailer", "trailer"}
	if got := codes(res); !slices.Equal(got, want) {
		t.Errorf("codes = %v, want %v: %+v", got, want, res.Findings)
	}
	for _, f := range res.Findings {
		if f.Path != "docs/commit.txt" {
			t.Errorf("prose was reported: %+v", f)
		}
	}
}

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
	want := []string{"trailer", "trailer", "trace-term", "trace-term", "home-path"}
	if got := codes(res); !slices.Equal(got, want) {
		t.Errorf("codes = %v, want %v", got, want)
	}
	if res.Findings[0].Path != "b.yml" || res.Findings[0].Line != 3 {
		t.Errorf("trailer finding = %+v", res.Findings[0])
	}
	if res.Findings[4].Path != "docs/a.md" || res.Findings[4].Line != 4 || res.Findings[4].Column != 5 {
		t.Errorf("home path finding = %+v", res.Findings[4])
	}
}

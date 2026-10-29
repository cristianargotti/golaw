package policy_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/policy"
)

func lines(entries []policy.Line) map[string]policy.Line {
	out := map[string]policy.Line{}
	for _, l := range entries {
		out[l.Key] = l
	}
	return out
}

func TestExplainFull(t *testing.T) {
	t.Parallel()
	root, d := enrolled(t, fullPolicy)
	l, err := policy.Load(t.Context(), input(root, d))
	if err != nil {
		t.Fatal(err)
	}
	got := lines(policy.Explain(l))
	want := map[string]policy.Line{
		"golaw":                         {Key: "golaw", Value: "1", Source: policy.SourceCore},
		"kit.version":                   {Key: "kit.version", Value: "0.1.0", Source: "HEAD:golaw.yml"},
		"profile.name":                  {Key: "profile.name", Value: "acme", Source: "HEAD:golaw.yml"},
		"language":                      {Key: "language", Value: "en-US", Source: "HEAD:golaw.yml"},
		"capabilities":                  {Key: "capabilities", Value: "[cli, go]", Source: "HEAD:golaw.yml"},
		"layout.main":                   {Key: "layout.main", Value: "root", Source: "HEAD:golaw.yml"},
		"checks.sizes.file_lines":       {Key: "checks.sizes.file_lines", Value: "250", Source: "HEAD:golaw.yml"},
		"checks.terms.tools":            {Key: "checks.terms.tools", Value: "[acme-tool, other]", Source: "HEAD:golaw.yml"},
		"checks.shell.deny_pr":          {Key: "checks.shell.deny_pr", Value: "false", Source: "HEAD:golaw.yml"},
		"checks.coverage.floors.total":  {Key: "checks.coverage.floors.total", Value: "85", Source: "HEAD:golaw.yml"},
		"checks.text.tool-terms.names":  {Key: "checks.text.tool-terms.names", Value: "[a, b]", Source: "HEAD:golaw.yml"},
		"checks.text.tool-terms.strict": {Key: "checks.text.tool-terms.strict", Value: "true", Source: "HEAD:golaw.yml"},
		"vulnerability_waivers[0]":      {Key: "vulnerability_waivers[0]", Value: "GO-2026-6603 until 2026-11-01 by docs/adr/0007-go-1-27-2.md", Source: "HEAD:golaw.yml"},
		"exceptions[1]":                 {Key: "exceptions[1]", Value: "text.dashes at docs/old.md until 2020-01-01 by docs/adr/0010-old.md", Source: "HEAD:golaw.yml"},
		"ladder.pre_push":               {Key: "ladder.pre_push", Value: "false", Source: "HEAD:golaw.yml"},
		"managed[0]":                    {Key: "managed[0]", Value: ".golangci.yml " + abHash, Source: "HEAD:golaw.yml"},
		"enrollment.state":              {Key: "enrollment.state", Value: "enrolled", Source: policy.SourceEnrollment},
		"enrollment.profile_digest":     {Key: "enrollment.profile_digest", Value: "sha256:" + acmeDigest, Source: policy.SourceEnrollment},
		"digest":                        {Key: "digest", Value: l.Policy.Digest, Source: policy.SourceComputed},
	}
	for key, line := range want {
		if got[key] != line {
			t.Errorf("%s = %+v, want %+v", key, got[key], line)
		}
	}
	if len(got) != 40 {
		t.Errorf("Explain printed %d lines", len(got))
	}
}

func TestExplainCore(t *testing.T) {
	t.Parallel()
	root := initRepo(t)
	write(t, root, "a.txt", "a\n")
	commitAll(t, root, "one")
	l, err := policy.Load(t.Context(), input(root, dirs(t)))
	if err != nil {
		t.Fatal(err)
	}
	got := lines(policy.Explain(l))
	for _, key := range []string{"kit.version", "profile", "language", "capabilities", "layout.main", "layout.decisions", "checks.sizes.function_lines", "checks.terms.allow_paths", "checks.shell.deny_push"} {
		if got[key].Source != policy.SourceCore {
			t.Errorf("%s = %+v, want the core default", key, got[key])
		}
	}
	if got["language"].Value != "pt-BR" || got["layout.decisions"].Value != "docs/adr" || got["enrollment.state"].Value != "unenrolled" {
		t.Errorf("core values = %+v", got)
	}
	var buf bytes.Buffer
	if err := policy.WriteExplain(&buf, policy.Explain(l)); err != nil || !strings.Contains(buf.String(), "language = pt-BR (core default)\n") {
		t.Errorf("WriteExplain = %q, %v", buf.String(), err)
	}
	if err := policy.WriteExplain(failingWriter{}, policy.Explain(l)); err == nil {
		t.Error("a failing writer is reported")
	}
}

// optionDefaults are the option lines of a policy that sets no option,
// each at its core default.
func optionDefaults() map[string]string {
	return map[string]string{
		"checks.coverage.floors.total":              "80",
		"checks.coverage.floors.package":            "80",
		"checks.coverage.floors.file":               "80",
		"checks.coverage.floors.min_statements":     "20",
		"checks.coverage.floors.file_exemptions":    "[]",
		"checks.coverage.floors.new_code":           "80",
		"checks.policy.golangci.allowed_exclusions": "[]",
		"checks.text.tool-terms.limit":              "5",
		"checks.text.tool-terms.names":              "[]",
		"checks.text.tool-terms.strict":             "false",
	}
}

func TestExplainListsEveryOptionAtItsDefault(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		text string
		set  map[string]string // the option lines the file sets, with their value
	}{
		"minimal": {minimalPolicy, map[string]string{}},
		"partial": {minimalPolicy + "checks: {coverage: {total: 90}, text.tool-terms: {limit: 3}}\n", map[string]string{
			"checks.coverage.floors.total": "90", "checks.text.tool-terms.limit": "3",
		}},
	}
	for name, c := range cases {
		root, d := enrolled(t, c.text)
		l, err := policy.Load(t.Context(), input(root, d))
		if err != nil {
			t.Fatal(err)
		}
		got := lines(policy.Explain(l))
		for key, value := range optionDefaults() {
			want := policy.Line{Key: key, Value: value, Source: policy.SourceCore}
			if set, ok := c.set[key]; ok {
				want = policy.Line{Key: key, Value: set, Source: "HEAD:golaw.yml"}
			}
			if got[key] != want {
				t.Errorf("%s: %s = %+v, want %+v", name, key, got[key], want)
			}
		}
		if len(got) != 29 {
			t.Errorf("%s: Explain printed %d lines", name, len(got))
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.ErrUnsupported }

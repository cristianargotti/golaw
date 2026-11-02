package goast_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
	"github.com/cristianargotti/golaw/internal/gen/goldentest"
)

// TestSonarLiteralRepeatAndTheTemplateLawAgree holds the S1192 template
// law to this check line by line: a template the law accepts must never
// render a file the kit's own scan then reports, and a law stricter than
// the check would refuse a sound template.
func TestSonarLiteralRepeatAndTheTemplateLawAgree(t *testing.T) {
	t.Parallel()
	var law goldentest.Law
	for _, candidate := range goldentest.Laws() {
		if candidate.Name == "S1192-literals" {
			law = candidate
		}
	}
	if law.Assert == nil {
		t.Fatal("no S1192-literals law")
	}
	sources := map[string]string{
		"internal/app/m.go":   messagesSource,
		"internal/app/row.go": tagsSource,
		"internal/app/db.go":  "package app\n\nvar dsns = []string{\"a: b c\", \"a: b c\", \"a: b c\", \"ab_cd_ef\", \"ab_cd_ef\", \"ab_cd_ef\"}\n",
	}
	for path, source := range sources {
		res := scan(t, goast.SonarLiteralRepeat(), map[string]string{path: source})
		want := make([]string, 0, len(res.Findings))
		for _, line := range lines(res) {
			want = append(want, path+":"+strconv.Itoa(line)+" S1192-literals: ")
		}
		got := law.Assert(map[string][]byte{path: []byte(source)})
		if len(got) != len(want) {
			t.Fatalf("%s: the law answers %q, the check reports lines %v", path, got, lines(res))
		}
		for i := range want {
			if !strings.HasPrefix(got[i], want[i]) {
				t.Errorf("%s: law message %q, want the check's place %q", path, got[i], want[i])
			}
		}
	}
}

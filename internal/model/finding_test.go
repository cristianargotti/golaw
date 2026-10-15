package model_test

import (
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestSeverityText(t *testing.T) {
	t.Parallel()
	for s := model.SeverityDeny; s <= model.SeverityNote; s++ {
		text, err := s.MarshalText()
		if err != nil {
			t.Fatal(err)
		}
		var back model.Severity
		if err := back.UnmarshalText(text); err != nil || back != s {
			t.Errorf("severity %s does not round trip: %v", s, err)
		}
		if !s.Valid() {
			t.Errorf("severity %s must be valid", s)
		}
	}
	var unknown model.Severity
	if err := unknown.UnmarshalText([]byte("warn")); !errors.Is(err, model.ErrUsage) {
		t.Errorf("unknown severity: %v", err)
	}
	if model.Severity(0).String() != "unknown" || model.Severity(0).Valid() {
		t.Error("the zero severity must be unknown and invalid")
	}
	if !(model.Finding{Severity: model.SeverityDeny}).Enforced() || (model.Finding{Severity: model.SeverityNote}).Enforced() {
		t.Error("Enforced is wrong")
	}
}

func TestFingerprintIgnoresLinesAndWhitespace(t *testing.T) {
	t.Parallel()
	a := model.Finding{Check: "text.dashes", Rule: "rules/3@1", Path: "docs/a.md", Code: "em-dash", Message: "em dash  in prose", Line: 3}
	b := a
	b.Line = 40
	b.Message = "em dash in\tprose"
	if model.Fingerprint(a) != model.Fingerprint(b) {
		t.Error("a line shift or a whitespace change must keep the fingerprint")
	}
	c := a
	c.Path = "docs/b.md"
	if model.Fingerprint(a) == model.Fingerprint(c) {
		t.Error("another path must change the fingerprint")
	}
	if len(model.Fingerprint(a)) != 64 {
		t.Errorf("fingerprint %q is not a sha256 hex", model.Fingerprint(a))
	}
}

func TestSortFindings(t *testing.T) {
	t.Parallel()
	findings := []model.Finding{
		{Path: "b.go", Line: 1, Check: "x.b", Code: "c", Message: "m"},
		{Path: "a.go", Line: 2, Check: "x.a", Code: "c", Message: "m"},
		{Path: "a.go", Line: 1, Column: 5, Check: "x.a", Code: "c", Message: "m"},
		{Path: "a.go", Line: 1, Column: 2, Check: "x.b", Code: "c", Message: "m"},
		{Path: "a.go", Line: 1, Column: 2, Check: "x.a", Code: "d", Message: "m"},
		{Path: "a.go", Line: 1, Column: 2, Check: "x.a", Code: "c", Message: "n"},
		{Path: "a.go", Line: 1, Column: 2, Check: "x.a", Code: "c", Message: "m", Fingerprint: "2"},
		{Path: "a.go", Line: 1, Column: 2, Check: "x.a", Code: "c", Message: "m", Fingerprint: "1"},
	}
	model.SortFindings(findings)
	want := []string{"a.go:1:2:x.a:c:m:1", "a.go:1:2:x.a:c:m:2", "a.go:1:2:x.a:c:n:", "a.go:1:2:x.a:d:m:", "a.go:1:2:x.b:c:m:", "a.go:1:5:x.a:c:m:", "a.go:2:0:x.a:c:m:", "b.go:1:0:x.b:c:m:"}
	for i, f := range findings {
		got := f.Path + ":" + itoa(f.Line) + ":" + itoa(f.Column) + ":" + f.Check + ":" + f.Code + ":" + f.Message + ":" + f.Fingerprint
		if got != want[i] {
			t.Errorf("position %d = %s, want %s", i, got, want[i])
		}
	}
}

func itoa(n int) string {
	return string(rune('0' + n))
}

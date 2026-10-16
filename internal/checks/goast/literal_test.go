package goast_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
	"github.com/cristianargotti/golaw/internal/report"
)

const messagesSource = `package app

import "fmt"

func f() []string {
	return []string{
		"order not found",
		"order not found",
		"order not found",
		"short",
		"short",
		"short",
		"simple_word",
		"simple_word",
		"simple_word",
		"twice only",
		"twice only",
		` + "`order not found`" + `,
		"caf\u00e9 x",
		"caf\u00e9 x",
		"caf\u00e9 x",
		fmt.Sprint("order not found"),
	}
}
`

func TestSonarLiteralRepeat(t *testing.T) {
	t.Parallel()
	res := scan(t, goast.SonarLiteralRepeat(), map[string]string{"internal/app/m.go": messagesSource})
	expectCodes(t, res, "literal-repeat", "literal-repeat")
	expectLines(t, res, 7, 19)
	if res.Findings[0].Message != "a string literal appears 4 times in the file: name it once" {
		t.Errorf("message = %q", res.Findings[0].Message)
	}
}

const tagsSource = `package app

type row struct {
	A string "json:\"order_id\""
	B string "json:\"order_id\""
	C struct {
		D string "json:\"order_id\""
	}
}

func f() any {
	return struct {
		E string "json:\"order_id\""
	}{}
}
`

func TestSonarLiteralRepeatNeverCountsAStructTag(t *testing.T) {
	t.Parallel()
	expectCodes(t, scan(t, goast.SonarLiteralRepeat(), map[string]string{"internal/app/row.go": tagsSource}))
}

func TestSonarLiteralRepeatKeepsTheValueOutOfTheReport(t *testing.T) {
	t.Parallel()
	const value = "dsn://app:synthetic-pass@db:5432/app"
	source := "package app\n\nvar dsns = []string{\"" + value + "\", \"" + value + "\", \"" + value + "\"}\n"
	res := scan(t, goast.SonarLiteralRepeat(), map[string]string{"internal/app/db.go": source})
	expectCodes(t, res, "literal-repeat")
	expectLines(t, res, 3)
	var text strings.Builder
	if err := report.WriteText(&text, report.Document{Findings: res.Findings}); err != nil {
		t.Fatal(err)
	}
	for _, out := range []string{res.Findings[0].Message, text.String()} {
		if strings.Contains(out, "synthetic-pass") {
			t.Errorf("the literal value reaches the report: %q", out)
		}
	}
}

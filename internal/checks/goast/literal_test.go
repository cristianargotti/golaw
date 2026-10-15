package goast_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
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
	if res.Findings[0].Message != `the literal "order not found" appears 4 times in the file: name it once` {
		t.Errorf("message = %q", res.Findings[0].Message)
	}
}

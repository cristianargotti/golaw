package goast_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
)

const noopSource = `package app

type noop struct{}

func (noop) Flush() {}

func (noop) Close() error { return nil }

func (noop) Reset() {
	// the state lives in the caller
}

func (noop) Sync() {} // nothing to sync

func external()

var hook = func() {}

var noted = func() { /* why */ }
`

func TestSonarEmptyBody(t *testing.T) {
	t.Parallel()
	res := scan(t, goast.SonarEmptyBody(), map[string]string{"internal/app/noop.go": noopSource})
	expectCodes(t, res, "empty-body", "empty-body")
	expectLines(t, res, 5, 17)
	if !strings.HasPrefix(res.Findings[0].Message, "function Flush has an empty body") ||
		!strings.HasPrefix(res.Findings[1].Message, "a function literal has an empty body") {
		t.Errorf("messages = %+v", res.Findings)
	}
}

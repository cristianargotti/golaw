package goast_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
)

const serviceSource = `package app

import (
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("not found")

var (
	ErrGone    = errors.New("gone")
	ErrWrapped = fmt.Errorf("wrapped: %w", ErrGone)
	limit      = 3
)

const name = "x"

func find() error {
	return errors.New("inside a function the linter answers")
}
`

func TestSentinelDecl(t *testing.T) {
	t.Parallel()
	res := scan(t, goast.SentinelDecl(), map[string]string{
		"internal/app/service.go": serviceSource,
		"internal/app/errors.go":  serviceSource,
	})
	expectCodes(t, res, "sentinel-decl", "sentinel-decl")
	expectLines(t, res, 8, 11)
	want := "a sentinel is declared outside errors.go: a package declares its failure modes once"
	if res.Findings[0].Path != "internal/app/service.go" || res.Findings[0].Message != want {
		t.Errorf("finding = %+v", res.Findings[0])
	}
}

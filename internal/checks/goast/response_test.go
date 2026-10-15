package goast_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
)

const handlerSource = `package h

import "net/http"

func f(c responder, legacy int) {
	c.JSON(http.StatusNotFound, nil)
	c.String(500, "boom")
	c.AbortWithStatusJSON(http.StatusTeapot, nil)
	c.JSON(http.StatusOK, nil)
	c.JSON(200, nil)
	c.JSON(legacy, nil)
	c.JSON("x", nil)
	c.Render(http.StatusNotFound, nil)
	c.JSON()
	c.JSON(http.StatusBadRequest, nil) // contract: mirrors the v1 envelope
	c.JSON(other.StatusNotFound, nil)
	respond(http.StatusNotFound)
}
`

func TestRawErrorResponse(t *testing.T) {
	t.Parallel()
	res := scan(t, goast.RawErrorResponse(), map[string]string{
		"internal/http/h.go":        handlerSource,
		"internal/http/errors/h.go": handlerSource,
		"internal/http/h_test.go":   handlerSource,
	})
	expectCodes(t, res, "raw-error-response", "raw-error-response", "raw-error-response")
	expectLines(t, res, 6, 7, 8)
	if !strings.HasPrefix(res.Findings[0].Message, "an error leaves the API outside the errors package") {
		t.Errorf("message = %q", res.Findings[0].Message)
	}
	if needs := goast.RawErrorResponse().Spec().Needs; len(needs) != 2 || needs[1] != "api" {
		t.Errorf("needs = %v, want go and api", needs)
	}
}

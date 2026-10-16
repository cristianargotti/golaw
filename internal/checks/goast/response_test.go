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

// encodedSource spells statuses in every base and with digit separators:
// 0x1f4, 5_00 and 0o764 are 500, 0500 is the octal 320 and 0x12c is 300.
const encodedSource = `package h

func f(c responder) {
	c.JSON(0x1f4, nil)
	c.String(5_00, "boom")
	c.Data(0o764, "text/plain", nil)
	c.JSON(0500, nil)
	c.JSON(0x12c, nil)
}
`

func TestRawErrorResponseReadsTheLiteralAsGoDoes(t *testing.T) {
	t.Parallel()
	res := scan(t, goast.RawErrorResponse(), map[string]string{"internal/http/h.go": encodedSource})
	expectCodes(t, res, "raw-error-response", "raw-error-response", "raw-error-response")
	expectLines(t, res, 4, 5, 6)
}

// markerSource clears a raw error only where the marker opens the comment
// and names the envelope.
const markerSource = `package h

import "net/http"

func f(c responder) {
	c.JSON(http.StatusNotFound, nil) // contract:
	c.JSON(http.StatusNotFound, nil) // breaks the contract: no envelope
	c.JSON(http.StatusNotFound, nil) /* contract: */
	c.JSON(http.StatusNotFound, nil) // contract: mirrors the v1 envelope
	c.JSON(http.StatusNotFound, nil) /* contract: mirrors the v1 envelope */
}
`

func TestRawErrorResponseNeedsAMarkerThatNamesTheEnvelope(t *testing.T) {
	t.Parallel()
	res := scan(t, goast.RawErrorResponse(), map[string]string{"internal/http/h.go": markerSource})
	expectCodes(t, res, "raw-error-response", "raw-error-response", "raw-error-response")
	expectLines(t, res, 6, 7, 8)
}

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

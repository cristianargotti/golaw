package handler_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/goldentest"
)

func TestHandlerTemplateLawsHoldBothWays(t *testing.T) {
	t.Parallel()
	goldentest.Prove(t, "testdata/proof.yml", goldentest.Laws())
}

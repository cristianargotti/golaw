package cursor_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/hooktest"
)

func TestCursorProof(t *testing.T) {
	t.Parallel()
	hooktest.Prove(t, "proof/manifest.yml", "cursor", curAdapter())
}

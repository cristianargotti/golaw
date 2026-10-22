package codex_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/hooktest"
)

func TestCodexProof(t *testing.T) {
	t.Parallel()
	hooktest.Prove(t, "proof/manifest.yml", "codex", adapter())
}

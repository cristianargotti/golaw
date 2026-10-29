package lint_test

import (
	"os"
	"testing"

	"github.com/cristianargotti/golaw/internal/runnertest"
)

// TestMain lets every test of the package, internal ones included, play a
// fake tool: the test binary is one for both test packages.
func TestMain(m *testing.M) { os.Exit(runnertest.Main(m)) }

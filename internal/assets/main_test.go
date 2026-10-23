package assets_test

import (
	"os"
	"testing"

	"github.com/cristianargotti/golaw/internal/runnertest"
)

func TestMain(m *testing.M) {
	os.Exit(runnertest.Main(m))
}

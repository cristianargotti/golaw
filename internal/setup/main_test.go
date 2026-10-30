package setup_test

import (
	"os"
	"testing"

	"github.com/cristianargotti/golaw/internal/runnertest"
)

func TestMain(m *testing.M) { os.Exit(runnertest.Main(m, os.Getenv)) }

package rung_test

import (
	"os"
	"testing"

	"github.com/cristianargotti/golaw/internal/runnertest"
)

// TestMain lets a copy of this test binary play a pinned tool, so a rung is
// tested without the real toolchain.
func TestMain(m *testing.M) { os.Exit(runnertest.Main(m, os.Getenv)) }

package main

import (
	"os"
	"testing"

	"github.com/cristianargotti/golaw/internal/runnertest"
)

// TestMain lets a copy of this test binary play a pinned tool, so the
// commands that run git hooks, setup and the ladder helpers are tested
// without the real tools.
func TestMain(m *testing.M) { os.Exit(runnertest.Main(m)) }

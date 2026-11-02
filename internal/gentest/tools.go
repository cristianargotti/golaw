package gentest

import (
	"runtime"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/toolchain"
	"github.com/cristianargotti/golaw/internal/toolchaintest"
)

// hookGenerators is how many pinned generators the post hook may run.
const hookGenerators = 2

// hookGenerator reports a generator the post hook may run (A44): goverter is
// left out because it loads the repository's packages, which a hook never
// does.
func hookGenerator(name string) bool { return name == "sqlc" || name == "oapi-codegen" }

// InstallGenerators puts fakes of the generators the hook runs in dir, with
// the receipts a real install writes, each answering b, and answers their
// paths by name: a test proves the hook ran one, killed one at the budget
// (b.Delay) or reported one missing (not installed). The calling package's
// TestMain hands its run to runnertest.Main, because each fake is the test
// binary started again.
func InstallGenerators(tb testing.TB, dir string, b runnertest.Behavior) map[string]string {
	tb.Helper()
	paths := make(map[string]string, hookGenerators)
	for _, g := range runner.Generators() {
		if !hookGenerator(g.Name) {
			continue
		}
		tool := toolchain.Tool{Name: g.Name, Version: g.Version, Distribution: toolchain.DistBinary}
		paths[g.Name] = toolchaintest.Install(tb, dir, tool, runtime.GOOS, runtime.GOARCH, b)
	}
	return paths
}

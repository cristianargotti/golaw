package toolchaintest_test

import (
	"runtime"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/hometest"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
	"github.com/cristianargotti/golaw/internal/toolchaintest"
)

func TestInstallForInstallsEveryToolTheLanguagesNeed(t *testing.T) {
	t.Parallel()
	tools := hometest.New(t).Tools()
	paths := toolchaintest.InstallFor(t, tools, []surface.Language{surface.Dockerfile}, runtime.GOOS, runtime.GOARCH, runnertest.Behavior{})
	names := make([]string, 0, len(paths))
	for name := range paths {
		names = append(names, name)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"checkov", "hadolint"}) {
		t.Fatalf("InstallFor installed %v", names)
	}
	for name, path := range paths {
		if receipt, found, err := toolchain.ReadReceipt(tools, name); err != nil || !found || receipt.Path != path {
			t.Errorf("receipt of %s = %+v, %t, %v", name, receipt, found, err)
		}
	}
}

func TestCallsReadsTheRecordOfAFakeByItsPath(t *testing.T) {
	t.Parallel()
	tools := hometest.New(t).Tools()
	hadolint, _ := toolchain.Lookup("hadolint")
	path := toolchaintest.Install(t, tools, hadolint, runtime.GOOS, runtime.GOARCH, runnertest.Behavior{})
	if calls := toolchaintest.Calls(t, path); len(calls) != 0 {
		t.Fatalf("a fresh fake has calls %v", calls)
	}
	if _, err := runner.Run(t.Context(), runner.Command{Name: path, Args: []string{"-"}}); err != nil {
		t.Fatal(err)
	}
	if calls := toolchaintest.Calls(t, path); len(calls) != 1 || !slices.Equal(calls[0], []string{"-"}) {
		t.Errorf("Calls = %v", calls)
	}
}

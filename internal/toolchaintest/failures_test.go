package toolchaintest_test

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/cristianargotti/golaw/internal/hometest"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/toolchain"
	"github.com/cristianargotti/golaw/internal/toolchaintest"
)

// fatalTB records a fatal call and ends the goroutine as testing does, so
// a helper's refusal is observed without failing the test that calls it.
type fatalTB struct {
	testing.TB

	mu     sync.Mutex
	failed bool
}

func (f *fatalTB) Helper() {}

func (f *fatalTB) Fatalf(string, ...any) {
	f.mu.Lock()
	f.failed = true
	f.mu.Unlock()
	runtime.Goexit()
}

// refused runs the helper call on its own goroutine and reports whether it
// stopped on a fatal call.
func refused(t *testing.T, call func(tb testing.TB)) bool {
	t.Helper()
	tb := &fatalTB{TB: t}
	var wg sync.WaitGroup
	wg.Go(func() { call(tb) })
	wg.Wait()
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.failed
}

func TestTheHelpersStopOnADirectoryTheyCannotWrite(t *testing.T) {
	t.Parallel()
	ruff, _ := toolchain.Lookup("ruff")
	t.Run("a file where the layout puts a directory", func(t *testing.T) {
		t.Parallel()
		tools := hometest.New(t).Tools()
		if err := os.MkdirAll(tools, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tools, "bin"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if !refused(t, func(tb testing.TB) {
			tb.Helper()
			toolchaintest.Unreceipted(tb, tools, ruff, runtime.GOOS, runtime.GOARCH, runnertest.Behavior{})
		}) {
			t.Error("Unreceipted went on past a directory it could not make")
		}
	})
	t.Run("a file where the receipts go", func(t *testing.T) {
		t.Parallel()
		tools := hometest.New(t).Tools()
		if err := os.MkdirAll(tools, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tools, "receipts"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if !refused(t, func(tb testing.TB) {
			tb.Helper()
			toolchaintest.Install(tb, tools, ruff, runtime.GOOS, runtime.GOARCH, runnertest.Behavior{})
		}) {
			t.Error("Install went on without its receipt")
		}
	})
}

func TestOnPathNamesABinaryToolByItsName(t *testing.T) {
	t.Parallel()
	bin := t.TempDir()
	ruff, _ := toolchain.Lookup("ruff")
	path := toolchaintest.OnPath(t, bin, ruff, "ruff 0.17.1", runnertest.Behavior{})
	want := filepath.Join(bin, "ruff")
	if runtime.GOOS == "windows" {
		want += ".exe"
	}
	if path != want {
		t.Errorf("OnPath = %s, want %s", path, want)
	}
}

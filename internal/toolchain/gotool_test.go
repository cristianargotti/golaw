package toolchain_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

func goTools() []runner.Tool {
	return []runner.Tool{
		{Name: "golangci-lint", Module: "github.com/golangci/golangci-lint/v2/cmd/golangci-lint", Version: "2.14.0", Probe: []string{"version"}},
		{Name: "sqlc", Module: "github.com/sqlc-dev/sqlc/cmd/sqlc", Version: "1.31.1", Probe: []string{"version"}},
	}
}

// fakeGo plays the go command: a build writes the file -o names, unless
// the test asks for a failure.
type fakeGo struct {
	mu     sync.Mutex
	calls  []runner.Command
	code   int
	stderr string
}

func (f *fakeGo) run(_ context.Context, c runner.Command) (runner.Output, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, c)
	if f.code != 0 {
		return runner.Output{Code: f.code, Stderr: []byte(f.stderr)}, nil
	}
	out := c.Args[slices.Index(c.Args, "-o")+1]
	return runner.Output{}, os.WriteFile(out, []byte("binary of "+c.Args[len(c.Args)-1]), 0o700) //nolint:wrapcheck // the fake plays the go command, whose error BuildGo wraps
}

func TestBuildGoBuildsEachToolFromTheToolsModuleWithAReceipt(t *testing.T) {
	t.Parallel()
	dir, root := t.TempDir(), t.TempDir()
	fake := &fakeGo{}
	var out bytes.Buffer
	o := toolchain.GoBuild{Dir: dir, Root: root, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Tools: goTools(), Run: fake.run, Out: &out}
	recs, err := toolchain.BuildGo(t.Context(), o)
	if err != nil || len(recs) != 2 {
		t.Fatalf("BuildGo = %+v, %v", recs, err)
	}
	for i, tool := range goTools() {
		path := toolchain.BinPath(dir, toolchain.GoTool(tool), runtime.GOOS, runtime.GOARCH)
		want := []string{"build", "-modfile=tools/go.mod", "-trimpath", "-o", path, tool.Module}
		if c := fake.calls[i]; c.Name != "go" || c.Dir != root || !slices.Equal(c.Args, want) {
			t.Errorf("call %d = %+v, want go %v in %s", i, c, want, root)
		}
		if rec, ok, err := toolchain.ReadReceipt(dir, tool.Name); !ok || err != nil || rec.Path != path || rec.Version != tool.Version {
			t.Errorf("receipt of %s = %+v, %v, %v", tool.Name, rec, ok, err)
		}
	}
	if !strings.Contains(out.String(), "sqlc 1.31.1 instalado e verificado em ") {
		t.Errorf("out = %q", out.String())
	}
	again := &fakeGo{}
	out.Reset()
	o.Run = again.run
	if _, err := toolchain.BuildGo(t.Context(), o); err != nil || len(again.calls) != 0 || !strings.Contains(out.String(), "já estava compilado e verificado") {
		t.Fatalf("second run: calls %d, err %v, out %q", len(again.calls), err, out.String())
	}
}

func TestBuildGoNamesTheToolThatFailed(t *testing.T) {
	t.Parallel()
	fake := &fakeGo{code: 1, stderr: "go: missing go.sum entry for module providing package github.com/sqlc-dev/sqlc/cmd/sqlc\n"}
	_, err := toolchain.BuildGo(t.Context(), toolchain.GoBuild{Dir: t.TempDir(), Root: t.TempDir(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Tools: goTools()[1:], Run: fake.run})
	if !errors.Is(err, toolchain.ErrInstall) || !strings.Contains(err.Error(), "sqlc 1.31.1") || !strings.Contains(err.Error(), "missing go.sum entry") {
		t.Fatalf("err = %v", err)
	}
	if _, err := toolchain.BuildGo(t.Context(), toolchain.GoBuild{Tools: goTools()}); !errors.Is(err, toolchain.ErrNoToolsDir) {
		t.Fatalf("no tools directory = %v", err)
	}
}

func TestGoToolIsTheBinaryLayoutOfAPin(t *testing.T) {
	t.Parallel()
	got := toolchain.GoTool(goTools()[1])
	if got.Name != "sqlc" || got.Version != "1.31.1" || got.Distribution != toolchain.DistBinary || !slices.Equal(got.Probe, []string{"version"}) {
		t.Fatalf("GoTool = %+v", got)
	}
}

// errNoGo is what runner.Run answers when PATH holds no go command.
var errNoGo = errors.New("exec: go: not found")

func TestBuildGoNamesAGoCommandThatDidNotStartOrSaidNothing(t *testing.T) {
	t.Parallel()
	start := func(context.Context, runner.Command) (runner.Output, error) {
		return runner.Output{}, errNoGo
	}
	silent := func(context.Context, runner.Command) (runner.Output, error) {
		return runner.Output{Code: 2, Stderr: []byte("\n  \n")}, nil
	}
	for want, run := range map[string]func(context.Context, runner.Command) (runner.Output, error){
		"exec: go: not found":         start,
		"go build exited with code 2": silent,
	} {
		_, err := toolchain.BuildGo(t.Context(), toolchain.GoBuild{Dir: t.TempDir(), Root: t.TempDir(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Tools: goTools()[:1], Run: run})
		if !errors.Is(err, toolchain.ErrInstall) || !strings.Contains(err.Error(), "golangci-lint 2.14.0") || !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want %q", err, want)
		}
	}
}

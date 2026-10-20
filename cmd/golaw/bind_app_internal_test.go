package main

import (
	"io"
	"runtime"
	"testing"

	"github.com/cristianargotti/golaw/internal/buildinfo"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// bindApp is the test app whose binding is the production one over a
// temporary home, for the commands that reach the binding through app.bind.
func bindApp(t *testing.T) testApp {
	t.Helper()
	ta := newTestApp(t)
	b, _ := testBinding(t)
	ta.bind = func() binding { return b }
	ta.buildInfo = func() buildinfo.Info { return b.info }
	return ta
}

func TestNewAppBindsTheProductionBinding(t *testing.T) {
	t.Parallel()
	a := newApp(io.Discard, io.Discard)
	if b := a.bind(); b.goos != runtime.GOOS || b.env == nil || b.environ == nil || b.home == nil || b.exe == nil {
		t.Errorf("binding = %+v", b)
	}
}

func TestBindAppReachesTheBindingOfItsHome(t *testing.T) {
	t.Parallel()
	ta := bindApp(t)
	p, err := ta.bind().load(t.Context(), repository(t, true), now())
	if err != nil || p.KitVersion != testVersion {
		t.Fatalf("policy = %+v, %v", p, err)
	}
	if ta.buildInfo().Version != testVersion {
		t.Errorf("build info = %+v", ta.buildInfo())
	}
}

func TestFakeToolsPlayInTheCommandTests(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := runnertest.Install(t, dir, "lefthook", runnertest.Behavior{Probe: []string{"version"}, Version: "2.1.12"})
	out, err := runner.Run(t.Context(), runner.Command{Name: path, Args: []string{"version"}, Dir: dir})
	if err != nil || out.Code != 0 || string(out.Stdout) != "2.1.12\n" {
		t.Errorf("fake lefthook = %+v, %v", out, err)
	}
}

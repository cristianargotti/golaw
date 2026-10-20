package main

import (
	"context"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"
)

type m2RoutingDriver struct {
	fixture      *suite
	preparation  *m2Script
	collection   *m2Script
	prepareMode  string
	prepared     bool
	preparations int
	m1Builds     int
	agentCalls   int
	agentLookups int
}

func m2NewRoutingDriver(t *testing.T) *m2RoutingDriver {
	t.Helper()
	return &m2RoutingDriver{fixture: m2TestSuite(t), prepareMode: "valid"}
}

func (d *m2RoutingDriver) lookup(t *testing.T) locator {
	t.Helper()
	return func(name string) (string, error) {
		if name != "go" && name != "git" && name != "sh" {
			d.agentLookups++
		}
		return d.fixture.lookup(name)
	}
}

func (d *m2RoutingDriver) execute(t *testing.T) executor {
	t.Helper()
	m1 := completeExecutor(t)
	return func(ctx context.Context, c command) result {
		if slices.Contains(c.args, "^TestM2Measurements$") {
			t.Fatal("functional routing launched measurements")
		}
		if c.timeout == 0 {
			if c.program == d.fixture.goTool && slices.Contains(c.args, "./cmd/golaw") {
				d.m1Builds++
			}
			if len(c.args) > 0 && c.args[0] == "agent" {
				d.agentCalls++
			}
			return m1(ctx, c)
		}
		if c.timeout != 20*time.Minute {
			t.Fatalf("functional M2 command timeout=%v", c.timeout)
		}
		return d.m2Execute(ctx, t, c)
	}
}

func (d *m2RoutingDriver) m2Execute(ctx context.Context, t *testing.T, c command) result {
	t.Helper()
	if d.preparation == nil {
		d.bind(t, c)
	}
	if d.prepared {
		return d.collection.Execute(ctx, c)
	}
	got := d.preparation.Execute(ctx, c)
	if c.program == d.fixture.binary && slices.Equal(c.args, []string{"version"}) {
		d.preparations++
		d.prepared = got.err == nil && got.code == 0
	}
	return got
}

func (d *m2RoutingDriver) bind(t *testing.T, c command) {
	t.Helper()
	home := environmentValue(c.env, "HOME")
	base := filepath.Dir(home)
	s := d.fixture
	s.root, s.base, s.home = c.dir, base, home
	s.binary = filepath.Join(base, "bin", executable(binaryName))
	s.env, s.m2OS = slices.Clone(c.env), runtime.GOOS
	s.m2 = &m2Shared{Base: base, Bin: filepath.Join(base, "tools")}
	d.preparation = m2PrepareScript(t, s, d.prepareMode)
	d.collection = m2CollectionScript(t, s, "answer")
}

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

func TestGenToolsFollowTheCapabilities(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		capabilities []string
		want         []string
	}{
		"red: every generator capability":  {[]string{"api", "db", "go", "hexagonal"}, []string{"golangci-lint", "goverter", "oapi-codegen", "sqlc"}},
		"red: a database only":             {[]string{"db", "go"}, []string{"golangci-lint", "sqlc"}},
		"near: the core builds the linter": {[]string{"go"}, []string{"golangci-lint"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			pol := model.CoreDefaults()
			pol.Capabilities = tc.capabilities
			got := make([]string, 0, len(tc.want))
			for _, tool := range genTools(&pol) {
				got = append(got, tool.Name)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("genTools = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSetupGenRefusesTheFlagsItDoesNotCombineWith(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	for _, tc := range []struct{ surface, uninstall bool }{{true, false}, {false, true}} {
		if err := a.setupGenCommand(t.Context(), &bytes.Buffer{}, nil, tc.surface, tc.uninstall); !errors.Is(err, model.ErrUsage) {
			t.Errorf("surface %v uninstall %v: err = %v", tc.surface, tc.uninstall, err)
		}
	}
}

func TestSetupGenBuildsIntoTheToolDirectory(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	root := repository(t, true)
	var calls []runner.Command
	run := func(_ context.Context, c runner.Command) (runner.Output, error) {
		calls = append(calls, c)
		return runner.Output{}, os.WriteFile(c.Args[slices.Index(c.Args, "-o")+1], []byte("binary"), 0o700)
	}
	var out bytes.Buffer
	if err := b.setupGen(t.Context(), &out, root, now(), run); err != nil {
		t.Fatal(err)
	}
	dirs, err := b.dirs()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := toolchain.ReadReceipt(dirs.Tools, "golangci-lint"); !ok || err != nil || len(calls) != 1 || calls[0].Dir != root {
		t.Fatalf("receipt %v %v, calls %+v", ok, err, calls)
	}
}

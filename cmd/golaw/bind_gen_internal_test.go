package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/buildinfo"
	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
)

// genApp is an app over a real repository of the growth fixture: the
// worktree snapshot of the binding and the fixture's golaw.yml compiled,
// so a test runs the commands without an enrollment record.
func genApp(t *testing.T) (testApp, *gittest.Repo) {
	t.Helper()
	repo := gentest.Module(t, gentest.Contracts())
	ta := newTestApp(t)
	b, _ := testBinding(t)
	ta.bind = func() binding { return b }
	ta.buildInfo = func() buildinfo.Info { return b.info }
	ta.discover = func(context.Context, string) (string, error) { return repo.Root, nil }
	ta.snapshot = b.snapshot
	ta.load = func(context.Context, string, time.Time) (*model.EffectivePolicy, error) {
		data, err := os.ReadFile(filepath.Join(repo.Root, "golaw.yml"))
		if err != nil {
			return nil, fmt.Errorf("read the fixture policy: %w", err)
		}
		return b.compile(data, now())
	}
	return ta, repo
}

func TestMainOfFollowsTheLayoutOfTheMainPackage(t *testing.T) {
	t.Parallel()
	root := &model.EffectivePolicy{Layout: model.Layout{Main: "root"}}
	cmd := &model.EffectivePolicy{Layout: model.Layout{Main: "cmd"}}
	main := []byte("package main\n\nfunc main() {}\n")
	tests := map[string]struct {
		policy *model.EffectivePolicy
		files  map[string][]byte
		want   string
		fails  bool
	}{
		"root without Go":                                     {root, map[string][]byte{"go.mod": []byte("module example.com/slice\n")}, "", false},
		"root with a main package":                            {root, map[string][]byte{"main.go": main}, "", false},
		"root with a library":                                 {root, map[string][]byte{"slice.go": []byte("package slice\n")}, "slice", false},
		"cmd with one main":                                   {cmd, map[string][]byte{"cmd/api/main.go": main}, "api", false},
		"cmd without a main":                                  {cmd, map[string][]byte{"cmd/README.md": []byte("tools\n")}, "slice", false},
		"cmd with two mains wires by hand":                    {cmd, map[string][]byte{"cmd/a/main.go": main, "cmd/b/main.go": main}, gen.MainByHand, false},
		"root without Go and mains under cmd wires by hand":   {root, map[string][]byte{"go.mod": []byte("module example.com/slice\n"), "cmd/server/main.go": main}, gen.MainByHand, false},
		"root without Go and a library under cmd":             {root, map[string][]byte{"go.mod": []byte("module example.com/slice\n"), "cmd/tools/tools.go": []byte("package tools\n")}, "", false},
		"red: a cmd main whose package clause does not parse": {cmd, map[string][]byte{"cmd/a/main.go": []byte("package\n")}, "", true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := mainOf(tc.policy, gentest.Snapshot(t, tc.files), "example.com/slice")
			if tc.fails != (err != nil) || got != tc.want || (tc.fails && !errors.Is(err, model.ErrUsage)) {
				t.Errorf("mainOf = %q, %v, want %q (fails %v)", got, err, tc.want, tc.fails)
			}
		})
	}
}

func TestModuleAndManifestOfTheSnapshot(t *testing.T) {
	t.Parallel()
	s := gentest.Snapshot(t, gentest.Contracts())
	if module, err := moduleOf(s); err != nil || module != "example.com/slice" {
		t.Errorf("moduleOf = %q, %v", module, err)
	}
	if m, err := manifestOf(s); err != nil || m.Schema != 1 || len(m.Entries) != 0 {
		t.Errorf("manifestOf an unenrolled tree = %+v, %v", m, err)
	}
	if _, err := moduleOf(gentest.Snapshot(t, map[string][]byte{"a.go": []byte("package a\n")})); !errors.Is(err, model.ErrUsage) {
		t.Errorf("moduleOf without go.mod = %v", err)
	}
	broken := gentest.Snapshot(t, map[string][]byte{".golaw/gen.yml": []byte("golaw-gen: 1\nunknown: key\n")})
	if _, err := manifestOf(broken); err == nil {
		t.Error("an invalid manifest answered no error")
	}
}

package bootstrap_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/gittest"
)

func TestReadModuleSortsDirectRequirementsAndIgnoresToolchain(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	data := "module example.test/service\n\ngo 1.24.0\n\ntoolchain go1.99.0\n\nrequire (\n" +
		" example.test/zebra v1.0.0\n example.test/middle v2.0.0+incompatible // indirect\n" +
		" example.test/alpha v0.2.0\n)\n"
	path := filepath.Join(root, "go.mod")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	env := append(gittest.Isolated(), "GOTOOLCHAIN=auto", "GOPROXY=off")
	before := append([]string(nil), env...)
	got, err := bootstrap.ReadModule(t.Context(), root, env)
	want := bootstrap.ModuleInfo{
		Path: "example.test/service", GoVersion: "1.24.0",
		Direct: []assets.Requirement{
			{Path: "example.test/alpha", Version: "v0.2.0"},
			{Path: "example.test/zebra", Version: "v1.0.0"},
		},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("module = %+v, want %+v, error %v", got, want, err)
	}
	if !reflect.DeepEqual(env, before) {
		t.Fatalf("environment changed: %q, want %q", env, before)
	}
	if after, readErr := os.ReadFile(path); readErr != nil || string(after) != data {
		t.Fatalf("module changed: %q, %v", after, readErr)
	}
}

func TestReadModuleFailuresPreserveInput(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, data string
		canceled   bool
	}{
		{name: "missing"},
		{name: "invalid", data: "module example.test/service\n\ngo malformed\n"},
		{name: "canceled", data: "module example.test/service\n\ngo 1.24.0\n", canceled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, path := moduleFailureInput(t, tc.data)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.canceled {
				cancel()
			}
			got, err := bootstrap.ReadModule(ctx, root, append(gittest.Isolated(), "GOPROXY=off"))
			if !errors.Is(err, bootstrap.ErrModule) || !reflect.DeepEqual(got, bootstrap.ModuleInfo{}) {
				t.Fatalf("module = %+v, error %v", got, err)
			}
			if tc.canceled && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation error = %v", err)
			}
			requireModuleInput(t, path, tc.data)
		})
	}
}

func moduleFailureInput(t *testing.T, data string) (string, string) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "go.mod")
	if data != "" {
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root, path
}

func requireModuleInput(t *testing.T, path, before string) {
	t.Helper()
	after, err := os.ReadFile(path)
	if before == "" {
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("missing module changed: %q, %v", after, err)
		}
		return
	}
	if err != nil || string(after) != before {
		t.Fatalf("module changed: %q, %v", after, err)
	}
}

func TestReadModuleWithoutGoDirectiveKeepsItAbsent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/service\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := bootstrap.ReadModule(t.Context(), root, append(gittest.Isolated(), "GOPROXY=off"))
	want := bootstrap.ModuleInfo{Path: "example.test/service"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("module = %+v, want %+v, error %v", got, want, err)
	}
}

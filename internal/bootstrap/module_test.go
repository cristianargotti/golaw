package bootstrap_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/gittest"
)

func TestReadModuleDoesNotSelectAnotherToolchain(t *testing.T) {
	t.Parallel()
	for _, version := range []string{"1.24.0", "1.27", "1.99.0"} {
		t.Run(version, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			data := "module example.test/service\n\ngo " + version +
				"\n\nrequire (\n example.test/direct v1.2.3\n" +
				" example.test/indirect v1.4.0 // indirect\n)\n"
			path := filepath.Join(root, "go.mod")
			if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			env := append(gittest.Isolated(), "GOTOOLCHAIN=auto", "GOPROXY=off")
			got, err := bootstrap.ReadModule(t.Context(), root, env)
			if err != nil {
				t.Fatal(err)
			}
			want := bootstrap.ModuleInfo{
				Path: "example.test/service", GoVersion: version,
				Direct:   []assets.Requirement{{Path: "example.test/direct", Version: "v1.2.3"}},
				Indirect: []assets.Requirement{{Path: "example.test/indirect", Version: "v1.4.0"}},
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("module = %+v, want %+v", got, want)
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != data {
				t.Fatalf("module was changed: %q, %v", after, err)
			}
		})
	}
}

package toolchain_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/cristianargotti/golaw/internal/hometest"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

func configTree() fstest.MapFS {
	return fstest.MapFS{
		"parity/ruff.toml":                 {Data: []byte("[lint]\nselect = [\"S105\"]\n")},
		"strict/ruff.toml":                 {Data: []byte("[lint]\nselect = [\"ALL\"]\n")},
		"strict/nested/pyrightconfig.json": {Data: []byte("{\"typeCheckingMode\": \"strict\"}\n")},
	}
}

func TestMaterializeWritesOneSetIntoTheToolDirectory(t *testing.T) {
	t.Parallel()
	tools := hometest.New(t).Tools()
	dir, err := toolchain.Materialize(tools, "0.3.0", "strict", configTree())
	if err != nil || dir != toolchain.ConfigDir(tools, "0.3.0", "strict") {
		t.Fatalf("Materialize = %q, %v", dir, err)
	}
	for name, want := range map[string]string{
		"ruff.toml":                 "[lint]\nselect = [\"ALL\"]\n",
		"nested/pyrightconfig.json": "{\"typeCheckingMode\": \"strict\"}\n",
	} {
		got, readErr := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if readErr != nil || string(got) != want {
			t.Errorf("%s = %q, %v, want %q", name, got, readErr, want)
		}
	}
	if _, err := os.Stat(toolchain.ConfigDir(tools, "0.3.0", "parity")); err == nil {
		t.Error("the parity set was written beside strict")
	}
}

func TestMaterializeRewritesOnlyWhatChanged(t *testing.T) {
	t.Parallel()
	tools := hometest.New(t).Tools()
	dir, err := toolchain.Materialize(tools, "0.3.0", "parity", configTree())
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "ruff.toml")
	old := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(file, old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := toolchain.Materialize(tools, "0.3.0", "parity", configTree()); err != nil {
		t.Fatal(err)
	}
	if info, statErr := os.Stat(file); statErr != nil || !info.ModTime().Equal(old) {
		t.Errorf("an unchanged file was rewritten: %v, %v", info, statErr)
	}
	changed := configTree()
	changed["parity/ruff.toml"] = &fstest.MapFile{Data: []byte("[lint]\nselect = [\"S105\", \"S106\"]\n")}
	if _, err := toolchain.Materialize(tools, "0.3.0", "parity", changed); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(file); err != nil || string(got) != "[lint]\nselect = [\"S105\", \"S106\"]\n" {
		t.Errorf("changed file = %q, %v", got, err)
	}
}

func TestMaterializeRefusesWhatItCannotPlace(t *testing.T) {
	t.Parallel()
	tools := hometest.New(t).Tools()
	if _, err := toolchain.Materialize(tools, "0.3.0", "lenient", configTree()); !errors.Is(err, toolchain.ErrConfig) {
		t.Errorf("unknown set = %v, want ErrConfig", err)
	}
	if _, err := toolchain.Materialize(tools, "0.3.0", "../strict", configTree()); !errors.Is(err, toolchain.ErrConfig) {
		t.Errorf("set with a path = %v, want ErrConfig", err)
	}
	if _, err := toolchain.Materialize("", "0.3.0", "strict", configTree()); !errors.Is(err, toolchain.ErrNoToolsDir) {
		t.Errorf("no tools directory = %v, want ErrNoToolsDir", err)
	}
}

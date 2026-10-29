package toolchain_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/hometest"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

func validReceipt(dir string) toolchain.Receipt {
	return toolchain.Receipt{
		Name: "ruff", Version: "0.17.1",
		Path:    filepath.Join(dir, "bin", "ruff", "0.17.1", "linux-amd64", "ruff"),
		SHA256:  strings.Repeat("ab", 32),
		Size:    26256176,
		ModTime: time.Date(2026, time.October, 27, 12, 0, 0, 0, time.UTC),
	}
}

func TestAReceiptRoundTripsThroughItsFile(t *testing.T) {
	t.Parallel()
	dir := hometest.New(t).Tools()
	want := validReceipt(dir)
	if err := toolchain.WriteReceipt(dir, want); err != nil {
		t.Fatalf("WriteReceipt = %v", err)
	}
	if path := toolchain.ReceiptPath(dir, "ruff"); path != filepath.Join(dir, "receipts", "ruff.json") {
		t.Errorf("ReceiptPath = %q", path)
	}
	got, found, err := toolchain.ReadReceipt(dir, "ruff")
	if err != nil || !found || got.Name != want.Name || got.SHA256 != want.SHA256 || got.Size != want.Size || !got.ModTime.Equal(want.ModTime) || got.Path != want.Path {
		t.Fatalf("ReadReceipt = %+v, %t, %v, want %+v", got, found, err, want)
	}
	if runtime.GOOS != "windows" {
		info, statErr := os.Stat(toolchain.ReceiptPath(dir, "ruff"))
		if statErr != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("receipt mode = %v, %v, want 0600", info, statErr)
		}
	}
}

func TestAnAbsentReceiptIsNotAnError(t *testing.T) {
	t.Parallel()
	got, found, err := toolchain.ReadReceipt(hometest.New(t).Tools(), "hadolint")
	if err != nil || found || got.Name != "" {
		t.Errorf("ReadReceipt = %+v, %t, %v, want nothing found", got, found, err)
	}
}

func TestAReceiptTheResolverCannotTrustIsRefused(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"not json":       "{",
		"another tool":   `{"name":"hadolint","version":"2.15.1","path":"/abs/hadolint","sha256":"` + strings.Repeat("ab", 32) + `","size":1,"mod_time":"2026-10-27T12:00:00Z"}`,
		"short checksum": `{"name":"ruff","version":"0.17.1","path":"/abs/ruff","sha256":"abc","size":1,"mod_time":"2026-10-27T12:00:00Z"}`,
		"no version":     `{"name":"ruff","version":"","path":"/abs/ruff","sha256":"` + strings.Repeat("ab", 32) + `","size":1,"mod_time":"2026-10-27T12:00:00Z"}`,
		"unknown field":  `{"name":"ruff","version":"0.17.1","path":"/abs/ruff","sha256":"` + strings.Repeat("ab", 32) + `","size":1,"mod_time":"2026-10-27T12:00:00Z","trusted":true}`,
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := hometest.New(t).Tools()
			path := toolchain.ReceiptPath(dir, "ruff")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := toolchain.ReadReceipt(dir, "ruff"); !errors.Is(err, toolchain.ErrReceipt) {
				t.Errorf("ReadReceipt = %v, want ErrReceipt", err)
			}
		})
	}
}

func TestWriteReceiptRefusesAReceiptItCouldNotReadBack(t *testing.T) {
	t.Parallel()
	tests := map[string]func(*toolchain.Receipt){
		"empty name":         func(r *toolchain.Receipt) { r.Name = "" },
		"name with a path":   func(r *toolchain.Receipt) { r.Name = "../ruff" },
		"relative tool path": func(r *toolchain.Receipt) { r.Path = "bin/ruff" },
		"no time":            func(r *toolchain.Receipt) { r.ModTime = time.Time{} },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := hometest.New(t).Tools()
			r := validReceipt(dir)
			mutate(&r)
			if err := toolchain.WriteReceipt(dir, r); !errors.Is(err, toolchain.ErrReceipt) {
				t.Errorf("WriteReceipt = %v, want ErrReceipt", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "receipts")); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("a refused receipt created the receipts directory: %v", err)
			}
		})
	}
}

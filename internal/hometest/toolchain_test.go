package hometest_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/hometest"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// The installer tests trust that a receipt and a configuration written
// under Tools() never leave the isolated home; this pins that promise
// against the layout the toolchain actually computes.
func TestTheToolchainWritesUnderTheTestHomeOnly(t *testing.T) {
	t.Parallel()
	h := hometest.New(t)
	ruff, ok := toolchain.Lookup("ruff")
	if !ok {
		t.Fatal("no pinned ruff")
	}
	path := toolchain.ExecPath(h.Tools(), ruff, "linux", "amd64")
	receipt := toolchain.Receipt{
		Name: ruff.Name, Version: ruff.Version, Path: path, SHA256: strings.Repeat("ab", 32),
		Size: 1, ModTime: time.Date(2026, time.October, 27, 0, 0, 0, 0, time.UTC),
	}
	if err := toolchain.WriteReceipt(h.Tools(), receipt); err != nil {
		t.Fatalf("WriteReceipt = %v", err)
	}
	for _, written := range []string{path, toolchain.ReceiptPath(h.Tools(), ruff.Name), toolchain.ConfigDir(h.Tools(), "", "strict")} {
		if rel, err := filepath.Rel(h.Data, written); err != nil || strings.HasPrefix(rel, "..") {
			t.Errorf("%q is outside the test home %q", written, h.Data)
		}
	}
}

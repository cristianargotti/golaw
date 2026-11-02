package goldentest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
)

// The floor step names what it could not do: a go.mod it cannot read and a
// toolchain the go command cannot reach, which is a skip and no failure.
func TestTheFloorStepNamesWhatItCouldNotDo(t *testing.T) {
	t.Parallel()
	t.Run("a module without its go.mod", func(t *testing.T) {
		t.Parallel()
		h := inGit(t)
		if err := os.Remove(filepath.Join(h.repo.Root, goModFile)); err != nil {
			t.Fatal(err)
		}
		h.floor(t.Context())
		if len(h.failures) != 1 || !strings.HasPrefix(h.failures[0], "floor: read go.mod: ") {
			t.Errorf("failures = %v", h.failures)
		}
	})
	t.Run("a toolchain the go command cannot reach", func(t *testing.T) {
		t.Parallel()
		h := inGit(t)
		h.floorBinary = func(context.Context, string) (string, string) { return "", "the toolchain is not reachable" }
		h.floor(t.Context())
		if reason := h.report.Skipped[StepFloor]; reason != "the toolchain is not reachable" || len(h.failures) != 0 {
			t.Errorf("skipped %q, failures %v", reason, h.failures)
		}
	})
}

// A go command that cannot start names its error, and a failed one the tail
// of what it printed.
func TestTheFloorBinaryNamesWhyItIsNotReachable(t *testing.T) {
	t.Parallel()
	if _, reason := floorBinary(t.Context(), filepath.Join(t.TempDir(), "absent")); !strings.HasPrefix(reason, "the "+floorToolchain+" toolchain is not reachable: ") {
		t.Errorf("reason = %q", reason)
	}
	if got := cause(runner.Output{Stdout: []byte("printed\n")}, nil); got != "printed" {
		t.Errorf("cause = %q", got)
	}
}

// A go.mod the step can neither lower nor put back fails twice, once for
// each write, and vets nothing.
func TestTheFloorStepNamesAGoModItCannotWrite(t *testing.T) {
	t.Parallel()
	h := inGit(t)
	h.floorBinary = func(context.Context, string) (string, string) { return "go", "" }
	if err := os.Chmod(filepath.Join(h.repo.Root, goModFile), 0o400); err != nil {
		t.Fatal(err)
	}
	h.floor(t.Context())
	if len(h.failures) != 2 || !strings.HasPrefix(h.failures[0], "floor: write go.mod: ") || !strings.HasPrefix(h.failures[1], "floor: restore go.mod: ") {
		t.Errorf("failures = %v", h.failures)
	}
}

package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// errClosed is the failure of an output nobody reads any more.
var errClosed = errors.New("output closed")

// closedWriter refuses every write, as a closed pipe does.
type closedWriter struct{}

func (closedWriter) Write([]byte) (int, error) { return 0, errClosed }

// A port the reader cannot judge without types is unmeasured, exit 2,
// never a pass and never a drift it could not prove.
func TestGenCheckIsUnmeasuredOverAPortThatNeedsTypes(t *testing.T) {
	t.Parallel()
	ta, root := newPort(t)
	ports := filepath.Join(root, "internal", "application", "growth", "ports.go")
	data, err := os.ReadFile(ports)
	if err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, root, "internal/application/growth/ports.go", strings.Replace(string(data), "type Ledger interface {", "type Ledger[T any] interface {", 1))
	if code := ta.run(t.Context(), []string{"gen", "--check"}); code != exitFailure || !strings.Contains(ta.stdout.String(), "golaw gen --check: unmeasured ") {
		t.Errorf("exit = %d, stdout %q", code, ta.stdout.String())
	}
	ta.app.stdout = closedWriter{}
	if code := ta.run(t.Context(), []string{"gen", "--check"}); code != exitFailure {
		t.Errorf("a closed output = %d, want exit 2", code)
	}
}

// A manifest entry whose template the kit does not embed stops both runs
// with exit 2 before anything is written.
func TestGenStopsAtATemplateTheKitDoesNotEmbed(t *testing.T) {
	t.Parallel()
	ta, root := newPort(t)
	path := filepath.Join(root, ".golaw", "gen.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, root, ".golaw/gen.yml", strings.Replace(string(data), "template: port@1", "template: port@9", 1))
	for _, args := range [][]string{{"gen"}, {"gen", "--check"}} {
		ta.stderr.Reset()
		if code := ta.run(t.Context(), args); code != exitFailure || !strings.Contains(ta.stderr.String(), "port@9") {
			t.Errorf("%q exit = %d, stderr %q", args, code, ta.stderr.String())
		}
	}
}

// A write that cannot land answers exit 2 and still prints what landed.
func TestGenNamesAWriteThatCannotLand(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("a directory without write permission refuses nothing to this user")
	}
	ta, root := newPort(t)
	dir := filepath.Join(root, "internal", "application", "growth", "ledgertest")
	if err := os.Remove(filepath.Join(dir, "zz_generated_fake.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:errcheck // TempDir removal reports a directory left read-only
	if code := ta.run(t.Context(), []string{"gen"}); code != exitFailure || !strings.Contains(ta.stderr.String(), "write: ") {
		t.Errorf("exit = %d, stderr %q", code, ta.stderr.String())
	}
}

func TestGenAndCheckAnswerAClosedOutput(t *testing.T) {
	t.Parallel()
	ta, _ := newPort(t)
	ta.app.stdout = closedWriter{}
	for _, args := range [][]string{{"gen"}, {"gen", "--json"}, {"gen", "--check", "--json"}} {
		if code := ta.run(t.Context(), args); code != exitFailure {
			t.Errorf("%q over a closed output = %d, want exit 2", args, code)
		}
	}
	ta.app.stderr = closedWriter{}
	ta.app.stdout = &strings.Builder{}
	if code := ta.run(t.Context(), []string{"gen"}); code != exitFailure {
		t.Errorf("a closed person channel = %d, want exit 2", code)
	}
}

func TestChangedOfIgnoresAFileOutsideTheRepository(t *testing.T) {
	t.Parallel()
	root, other := t.TempDir(), t.TempDir()
	writeRepoFile(t, other, "plans.sql", "-- name: ReadPlan :one\n")
	if got := changedOf(root, filepath.Join(other, "plans.sql")); got != nil {
		t.Errorf("changedOf = %v, want nil", got)
	}
}

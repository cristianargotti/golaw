//go:build windows

package hook_test

import (
	"errors"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// shortName is the path spelled with the 8.3 short name of each element
// that has one, as a runner's TEMP spells its profile, or skips the test
// where no element of the path has one.
func shortName(t *testing.T, path string) string {
	t.Helper()
	long, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, syscall.MAX_PATH)
	n, err := syscall.GetShortPathName(long, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || n >= uint32(len(buf)) {
		t.Skipf("no short name for %s: %v", path, err)
	}
	short := syscall.UTF16ToString(buf[:n])
	if strings.EqualFold(short, path) {
		t.Skipf("no element of %s has a short name", path)
	}
	return short
}

// TestRelativeReadsShortNamesAsTheLongOnes names a candidate the same way
// whether its directories are spelled by their long names, as the root
// discovery gives it, or by their 8.3 short names, as the TEMP of a runner
// spells the session's working directory, and names an element spelled
// short below the root by its long name, so the rules read the name the
// file has.
func TestRelativeReadsShortNamesAsTheLongOnes(t *testing.T) {
	t.Parallel()
	root := filepath.Join(realDir(t), "repository with a long name")
	secret := filepath.Join(root, "secrets with a long name", ".env")
	touch(t, secret)
	short, inner := shortName(t, root), shortName(t, filepath.Dir(secret))
	at := hook.Base{Root: root}
	for _, tc := range []struct{ cwd, name, want string }{
		{short, "a.go", "a.go"},
		{root, filepath.Join(short, "dir", "b.go"), "dir/b.go"},
		{root, filepath.Join(inner, ".env"), "secrets with a long name/.env"},
		{root, shortName(t, secret), "secrets with a long name/.env"},
		{short, filepath.Join(filepath.Base(inner), "new.go"), "secrets with a long name/new.go"},
	} {
		if got, err := at.Relative(tc.cwd, tc.name); err != nil || got != tc.want {
			t.Errorf("%s from %s: %q, %v, want %q", tc.name, tc.cwd, got, err, tc.want)
		}
	}
	if got, err := (hook.Base{Root: short}).Relative(root, "c.go"); err != nil || got != "c.go" {
		t.Errorf("a root spelled short: %q, %v", got, err)
	}
	if got := at.Resolve(short, filepath.Join(inner, ".env")); got != secret {
		t.Errorf("Resolve: %q, want %q", got, secret)
	}
}

// junction makes a junction of Windows, or skips the test where cmd cannot.
func junction(t *testing.T, target, name string) {
	t.Helper()
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", name, target).CombinedOutput(); err != nil {
		t.Skipf("junctions are not available: %v: %s", err, out)
	}
}

// TestJunctionIsFollowedAsALink follows a junction as a symbolic link: a
// name through one is named from the root it leads to, a junction of the
// repository that leads out of it carries a write outside, and enrollment
// is also looked up in the directory that holds the junction.
func TestJunctionIsFollowedAsALink(t *testing.T) {
	t.Parallel()
	repo, outside, elsewhere := realDir(t), realDir(t), realDir(t)
	junction(t, repo, filepath.Join(outside, "into"))
	junction(t, elsewhere, filepath.Join(repo, "out"))
	at := hook.Base{Root: repo}
	if got, err := at.Relative(filepath.Join(outside, "into"), "a.go"); err != nil || got != "a.go" {
		t.Errorf("a working directory through a junction: %q, %v", got, err)
	}
	if got, err := at.Relative(outside, filepath.Join(outside, "into", "dir", "b.go")); err != nil || got != "dir/b.go" {
		t.Errorf("a name through a junction: %q, %v", got, err)
	}
	if got, err := at.Relative(repo, filepath.Join(repo, "out", "c.go")); !errors.Is(err, model.ErrCannotJudge) {
		t.Errorf("a junction of the repository to outside: %q, %v, want cannot judge", got, err)
	}
	want := []string{repo, outside}
	if got := enrollmentCalls(t, model.ShapeWrite, outside, filepath.Join(outside, "into", "new.md")); !slices.Equal(got, want) {
		t.Errorf("enrolled from %v, want %v", got, want)
	}
}

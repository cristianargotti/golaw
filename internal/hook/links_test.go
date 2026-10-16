package hook_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// realDir is a temporary directory by its real path, the form enrollment
// is looked up in once every link of a target is followed.
func realDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// link makes a symbolic link, or skips the test where the system has none.
func link(t *testing.T, target, name string) {
	t.Helper()
	if err := os.Symlink(target, name); err != nil {
		t.Skipf("symbolic links are not available: %v", err)
	}
}

// touch writes a small file the links of a test lead to.
func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// enrollmentCalls evaluates an event over a target and answers the
// directories enrollment was looked up in, in order; each one answers
// unenrolled, so the whole list is tried.
func enrollmentCalls(t *testing.T, shape model.EventShape, cwd, target string) []string {
	t.Helper()
	req := request("bad")
	req.Event.Shape, req.Event.Cwd, req.Event.Target = shape, cwd, target
	dependencies := harnessDependencies(t, cwd)
	var calls []string
	dependencies.Enroll = func(_ context.Context, dir, _ string) (hook.Repository, error) {
		calls = append(calls, dir)
		return hook.Repository{Enrollment: model.Enrollment{State: model.EnrollmentNone}}, nil
	}
	_, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), harnessAdapter(req), dependencies)
	if got.Outcome != hook.Allow {
		t.Fatalf("%s: %+v", target, got)
	}
	return calls
}

// TestEnrollmentFollowsEveryLinkOfTheTarget looks enrollment up where the
// target of a write or read lands once every link of its name is followed,
// the last one included, dangling or not, and then where the name itself
// lives, so a link never moves a write or a read out of an enrolled
// repository in either direction; a read then falls back to the session.
func TestEnrollmentFollowsEveryLinkOfTheTarget(t *testing.T) {
	t.Parallel()
	repo, outside, session := realDir(t), realDir(t), realDir(t)
	touch(t, filepath.Join(repo, "golaw.yml"))
	touch(t, filepath.Join(outside, "plain.md"))
	relative, err := filepath.Rel(outside, filepath.Join(repo, "golaw.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{
		{filepath.Join(repo, "golaw.yml"), filepath.Join(outside, "alias")},
		{filepath.Join(repo, "fresh.md"), filepath.Join(outside, "dangling.md")},
		{relative, filepath.Join(outside, "relative")},
		{filepath.Join(outside, "alias"), filepath.Join(outside, "chain")},
		{filepath.Join(outside, "plain.md"), filepath.Join(repo, "out.md")},
		{filepath.Join(outside, "absent.md"), filepath.Join(repo, "gone.md")},
		{filepath.Join(outside, "loop-b"), filepath.Join(outside, "loop-a")},
		{filepath.Join(outside, "loop-a"), filepath.Join(outside, "loop-b")},
	} {
		link(t, pair[0], pair[1])
	}
	for _, tc := range []struct {
		name   string
		shape  model.EventShape
		target string
		want   []string
	}{
		{"an alias outside to the policy", model.ShapeWrite, filepath.Join(outside, "alias"), []string{repo, outside}},
		{"a dangling alias into the repository", model.ShapeWrite, filepath.Join(outside, "dangling.md"), []string{repo, outside}},
		{"a relative link", model.ShapeWrite, filepath.Join(outside, "relative"), []string{repo, outside}},
		{"a chain of links", model.ShapeWrite, filepath.Join(outside, "chain"), []string{repo, outside}},
		{"a link of the repository to a file outside", model.ShapeWrite, filepath.Join(repo, "out.md"), []string{outside, repo}},
		{"a dangling link of the repository", model.ShapeWrite, filepath.Join(repo, "gone.md"), []string{outside, repo}},
		{"a loop of links", model.ShapeWrite, filepath.Join(outside, "loop-a"), []string{outside}},
		{"a read through an alias", model.ShapeRead, filepath.Join(outside, "alias"), []string{repo, outside, session}},
		{"a plain read in the session", model.ShapeRead, filepath.Join(session, "notes.md"), []string{session}},
	} {
		if got := enrollmentCalls(t, tc.shape, session, tc.target); !slices.Equal(got, tc.want) {
			t.Errorf("%s: enrolled from %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestRequestedSpellsTheNameBelowTheRoot names a read as it was asked for:
// the path below the outermost directory that is the root, through
// whatever links lead there, so a directory link inside the repository
// keeps its own name, a link back to the root keeps every element below
// the repository boundary and a link outside that leads to the root names
// the file from it; a name no directory of the root holds is outside the
// repository.
func TestRequestedSpellsTheNameBelowTheRoot(t *testing.T) {
	t.Parallel()
	repo, outside := realDir(t), realDir(t)
	touch(t, filepath.Join(repo, "plain", "x.txt"))
	link(t, "plain", filepath.Join(repo, "vault"))
	link(t, repo, filepath.Join(outside, "proj"))
	link(t, ".", filepath.Join(repo, "here"))
	link(t, "..", filepath.Join(repo, "plain", "up"))
	for _, tc := range []struct{ cwd, name, want string }{
		{repo, filepath.Join(repo, "vault", "x.txt"), "vault/x.txt"},
		{outside, filepath.Join(outside, "proj", "notes.md"), "notes.md"},
		{repo, filepath.Join("vault", "x.txt"), "vault/x.txt"},
		{repo, filepath.Join(repo, "here", "plain", "x.txt"), "here/plain/x.txt"},
		{repo, filepath.Join(repo, "plain", "up", "here", "notes.md"), "plain/up/here/notes.md"},
		{outside, filepath.Join(outside, "proj", "here", "notes.md"), "here/notes.md"},
	} {
		if got, err := hook.Requested(repo, tc.cwd, tc.name); err != nil || got != tc.want {
			t.Errorf("%s: %q, %v, want %q", tc.name, got, err, tc.want)
		}
	}
	for _, tc := range []struct{ root, name string }{
		{repo, filepath.Join(outside, "plain.md")},
		{repo, ""},
		{repo, repo},
		{filepath.Join(outside, "absent-root"), filepath.Join(outside, "a.md")},
	} {
		if got, err := hook.Requested(tc.root, repo, tc.name); !errors.Is(err, model.ErrCannotJudge) {
			t.Errorf("%s under %s: %q, %v, want cannot judge", tc.name, tc.root, got, err)
		}
	}
	if got := hook.Resolve(outside, filepath.Join("proj", "plain", "x.txt")); got != filepath.Join(repo, "plain", "x.txt") {
		t.Errorf("Resolve through a linked directory: %q", got)
	}
	if got := hook.Resolve("relative", "x.txt"); got != filepath.Join("relative", "x.txt") {
		t.Errorf("a relative path has no place to start from: %q", got)
	}
}

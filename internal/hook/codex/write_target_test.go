package codex_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/codex"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
)

// Enrollment is read at the target of an event: a patch whose first hunk
// lies outside the working directory must not lend its location to the
// hunks inside it, or an unenrolled first path would allow them unjudged.
func TestPatchTargetIsTheFirstPathUnderTheWorkingDirectory(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, body, want string
	}{
		{"first local", "*** Add File: notes.md\n+x\n*** Add File: ../outside.md\n+x\n", "notes.md"},
		{"outside first", "*** Add File: ../outside.md\n+x\n*** Update File: main.go\n@@\n-a\n+b\n", "main.go"},
		{"moved after outside", "*** Delete File: ../outside.md\n*** Update File: old.md\n*** Move to: new.md\n@@\n-a\n+b\n", "old.md"},
		{"moved in from outside", "*** Update File: ../outside.md\n*** Move to: notes.md\n@@\n-a\n+b\n", "notes.md"},
		{"none local", "*** Add File: ../outside.md\n+x\n", "../outside.md"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := commandPayload(t, "pre-apply-patch-add.json", patch(tt.body))
			parsed, err := codex.Parse(data)
			if err != nil || parsed.Event.Target != tt.want {
				t.Fatalf("target = %q, %v; want %q", parsed.Event.Target, err, tt.want)
			}
		})
	}
}

// An absolute header under the working directory is local to it, and the
// other files of the patch are its further targets, so enrollment reads
// every file the patch writes.
func TestPatchTargetCountsAnAbsolutePathUnderTheWorkingDirectory(t *testing.T) {
	t.Parallel()
	root, outside := hooktest.Repository(t), hooktest.Repository(t)
	scratch := filepath.ToSlash(filepath.Join(outside, "outside.md"))
	inside := filepath.ToSlash(filepath.Join(root, "notes.md"))
	body := "*** Add File: " + scratch + "\n+x\n*** Add File: " + inside + "\n+x\n"
	parsed, err := codex.Parse(hooktest.Payload(commandPayload(t, "pre-apply-patch-add.json", patch(body)), root))
	if err != nil || parsed.Event.Target != inside {
		t.Fatalf("target = %q, %v; want %q", parsed.Event.Target, err, inside)
	}
	if len(parsed.Targets) != 1 || parsed.Targets[0] != scratch {
		t.Fatalf("further targets = %q, want [%q]", parsed.Targets, scratch)
	}
}

// enrolledUnder binds the proof repository at root and answers every
// directory outside it as an unenrolled location, so a test sees where
// enrollment governs a patch.
func enrolledUnder(t *testing.T, root string, files map[string][]byte) hook.Dependencies {
	t.Helper()
	dependencies := hooktest.Dependencies(t, root, files)
	enrolled := dependencies.Enroll
	dependencies.Enroll = func(ctx context.Context, dir, cwd string) (hook.Repository, error) {
		if rel, err := filepath.Rel(root, dir); err == nil && filepath.IsLocal(rel) {
			return enrolled(ctx, dir, cwd)
		}
		return hook.Repository{Enrollment: model.Enrollment{State: model.EnrollmentNone}}, nil
	}
	return dependencies
}

// A patch whose files lie both outside every repository and inside the
// enrolled one is never allowed unjudged: the core enrolls it where it
// writes the repository and refuses the file outside with a reason that
// names it and the fix; a patch whose every file lies outside keeps the
// valid allow of an unenrolled location.
func TestMixedPatchIsNeverAllowedUnjudged(t *testing.T) {
	t.Parallel()
	dash := string(rune(0x2014))
	root, outside := hooktest.Repository(t), hooktest.Repository(t)
	scratch := filepath.ToSlash(filepath.Join(outside, "scratch.md"))
	files := map[string][]byte{"main.go": []byte("package main\n"), "sub/keep.md": []byte("keep\n")}
	for _, tt := range []struct {
		name, cwd, body string
		want            hook.Outcome
		named           bool
	}{
		{"red absolute outside first", "/project", "*** Add File: " + scratch + "\n+x\n*** Update File: main.go\n@@\n-package main\n+package app\n", hook.Deny, true},
		{"red parent relative from a subdirectory", "/project/sub", "*** Add File: " + scratch + "\n+x\n*** Update File: ../main.go\n@@\n-package main\n+package app\n", hook.Deny, true},
		{"red moved in from outside", "/project", "*** Update File: " + scratch + "\n*** Move to: notes.md\n@@\n-x\n+y\n", hook.Deny, true},
		{"near every file outside", "/project", "*** Add File: " + scratch + "\n+hello " + dash + " world\n", hook.Allow, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := rewrite(t, commandPayload(t, "pre-apply-patch-add.json", patch(tt.body)), func(value map[string]any) {
				value["cwd"] = tt.cwd
			})
			_, result := hook.Evaluate(t.Context(), "codex", hooktest.Payload(data, root), adapter(), enrolledUnder(t, root, files))
			if result.Outcome != tt.want || result.CannotJudge != tt.named {
				t.Fatalf("want %s, got %+v", tt.want, result)
			}
			named := strings.Contains(result.Reason, scratch) && strings.Contains(result.Reason, "apply it in a separate patch")
			if named != tt.named {
				t.Fatalf("reason %q, named = %v", result.Reason, named)
			}
		})
	}
}

// A patch that selects a Codex environment applies where the hook cannot
// read, so it is cannot judge with the fix named, never judged against
// the local files; the same patch without the line is judged as usual.
func TestPatchThatSelectsAnEnvironmentCannotJudge(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, body string
		cannot     bool
	}{
		{"red remote environment", "*** Environment ID: remote\n*** Add File: notes.md\n+hello\n", true},
		{"near no environment", "*** Add File: notes.md\n+hello\n", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := commandPayload(t, "pre-apply-patch-add.json", patch(tt.body))
			_, result := hooktest.Evaluate(t, "codex", adapter(), data, nil)
			if result.CannotJudge != tt.cannot || (result.Outcome == hook.Allow) == tt.cannot {
				t.Fatalf("%+v", result)
			}
			if tt.cannot && (!strings.Contains(result.Reason, "remote") || !strings.Contains(result.Reason, "Environment ID")) {
				t.Fatalf("environment not named: %q", result.Reason)
			}
		})
	}
}

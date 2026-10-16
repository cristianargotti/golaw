package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
)

// TestHookTakesTheEnrolledPolicyBeforeItsFirstCommit drives the real
// binding in a repository with a history whose HEAD holds no golaw.yml
// yet, the policy init --apply wrote enrolled: a dash is judged and denied
// by its rule and a clean write is allowed, and once the checkout holds a
// policy the record does not, every write is denied with the reason that
// asks for the commit.
func TestHookTakesTheEnrolledPolicyBeforeItsFirstCommit(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	root := repository(t, false)
	gitRun(t, root, "add", "go.mod")
	gitRun(t, root, "commit", "-q", "-m", "build: start the module")
	data := renderedPolicy(t)
	p, err := b.compile(data, now())
	if err != nil {
		t.Fatal(err)
	}
	if err = b.enroll(root, data, p, now()); err != nil {
		t.Fatal(err)
	}
	deps, err := b.hookDependencies(agentUnderTest(), now())
	if err != nil {
		t.Fatal(err)
	}
	agent := agentUnderTest()
	write := func(content string) string {
		event := eventAt(t, "Write", root, map[string]any{"file_path": filepath.Join(root, "notes.md"), "content": content})
		reply := hook.Run(t.Context(), agent.ID, strings.NewReader(event), agent.Adapter, deps)
		if reply.ExitCode != 0 || reply.Stderr != "" {
			t.Fatalf("exit %d, stderr %q", reply.ExitCode, reply.Stderr)
		}
		return string(reply.Stdout)
	}
	if said := write("one \u2014 two\n"); !strings.Contains(said, `"permissionDecision":"deny"`) || !strings.Contains(said, "text.dashes") {
		t.Fatalf("a dash before the first commit: %s", said)
	}
	if said := write("one-two\n"); said != "" {
		t.Fatalf("a clean write before the first commit: %s", said)
	}
	weakened := bytes.Replace(data, []byte("file_lines: 300"), []byte("file_lines: 301"), 1)
	if err := os.WriteFile(filepath.Join(root, "golaw.yml"), weakened, 0o600); err != nil {
		t.Fatal(err)
	}
	if said := write("one-two\n"); !strings.Contains(said, `"permissionDecision":"deny"`) || !strings.Contains(said, "ask the person to commit golaw.yml") {
		t.Fatalf("a checkout policy the record does not hold: %s", said)
	}
}

// TestCommittedAsksHeadForTheEnrolledPolicy answers whether HEAD holds the
// golaw.yml of a digest: an unborn branch and a HEAD without the file hold
// none, a HEAD with the file holds its digest and no other, and a
// directory outside git is an error.
func TestCommittedAsksHeadForTheEnrolledPolicy(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	root := repository(t, false)
	p, err := b.compile(renderedPolicy(t), now())
	if err != nil {
		t.Fatal(err)
	}
	if committed, err := b.committed(t.Context(), root, p.Digest); err != nil || committed {
		t.Fatalf("an unborn branch = %v, %v", committed, err)
	}
	gitRun(t, root, "add", "go.mod")
	gitRun(t, root, "commit", "-q", "-m", "build: start the module")
	if committed, err := b.committed(t.Context(), root, p.Digest); err != nil || committed {
		t.Fatalf("a HEAD without the file = %v, %v", committed, err)
	}
	gitRun(t, root, "add", "golaw.yml")
	gitRun(t, root, "commit", "-q", "-m", "build: adopt the policy")
	if committed, err := b.committed(t.Context(), root, p.Digest); err != nil || !committed {
		t.Fatalf("a HEAD with the file = %v, %v", committed, err)
	}
	if committed, err := b.committed(t.Context(), root, "another"); err != nil || committed {
		t.Fatalf("another digest = %v, %v", committed, err)
	}
	if _, err := b.committed(t.Context(), t.TempDir(), p.Digest); err == nil {
		t.Fatal("a directory outside git was answered")
	}
}

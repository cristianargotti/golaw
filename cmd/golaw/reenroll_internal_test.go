package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// commitPolicy commits new bytes of golaw.yml in a repository with commits.
func commitPolicy(t *testing.T, root string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, policy.FileName), data, 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", policy.FileName)
	gitRun(t, root, "commit", "-q", "-m", "build: review the policy")
}

// sized is the rendered policy with another file length floor.
func sized(t *testing.T, lines string) []byte {
	t.Helper()
	return bytes.Replace(renderedPolicy(t), []byte("file_lines: 300"), []byte("file_lines: "+lines), 1)
}

// TestInstallReenrollsACommittedTightening is the repair of a reviewed
// policy change (A3): scan refuses the committed tightening the record does
// not hold, agent install answers the options of the policy in force, the
// port writes the record with its digest and its bytes, and the policy
// loads again; a later commit that loosens the tightened floor, even back
// toward the first render, is refused naming the floor.
func TestInstallReenrollsACommittedTightening(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	root := repository(t, true)
	data := renderedPolicy(t)
	p, err := b.compile(data, now())
	if err != nil {
		t.Fatal(err)
	}
	if err = b.enroll(root, data, p, now()); err != nil {
		t.Fatal(err)
	}
	commitPolicy(t, root, sized(t, "200"))
	if _, err = b.load(t.Context(), root, now()); !errors.Is(err, model.ErrEnrollment) || !strings.Contains(err.Error(), "run agent install") {
		t.Fatalf("a committed tightening loaded before its enrollment: %v", err)
	}
	options, err := b.installOptions(t.Context(), agentUnderTest(), root, scopeUser, now())
	if err != nil || options.PolicyDigest == p.Digest {
		t.Fatalf("options of the tightening = %+v, %v", options, err)
	}
	port, identity := enrolledPort(t, b, root)
	if _, err = port.WriteEnrollment(t.Context(), identity, options.PolicyDigest, options.ProfileDigest); err != nil {
		t.Fatal(err)
	}
	loaded, err := b.load(t.Context(), root, now())
	if err != nil || loaded.Sizes.FileLines != 200 || loaded.Enrollment.State != model.EnrollmentEnrolled {
		t.Fatalf("the re-enrolled policy = %+v, %v", loaded, err)
	}
	commitPolicy(t, root, sized(t, "250"))
	_, err = b.installOptions(t.Context(), agentUnderTest(), root, scopeUser, now())
	if !errors.Is(err, model.ErrEnrollment) || !strings.Contains(err.Error(), "file_lines") || strings.Contains(err.Error(), "commit the reviewed") {
		t.Fatalf("a committed weakening of the enrolled policy = %v", err)
	}
}

// TestEnrollRefusesAWeakeningOfTheRecord holds init --apply to the same
// rule: a record that enrolled a tightened policy is never moved back to
// the defaults, and the refused record keeps its digest.
func TestEnrollRefusesAWeakeningOfTheRecord(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	root := repository(t, false)
	tight := sized(t, "200")
	pt, err := b.compile(tight, now())
	if err != nil {
		t.Fatal(err)
	}
	if err = b.enroll(root, tight, pt, now()); err != nil {
		t.Fatal(err)
	}
	data := renderedPolicy(t)
	p, err := b.compile(data, now())
	if err != nil {
		t.Fatal(err)
	}
	if err = b.enroll(root, data, p, now()); !errors.Is(err, model.ErrEnrollment) || !strings.Contains(err.Error(), "weakens") {
		t.Fatalf("init enrolled a weakening: %v", err)
	}
	dirs, err := b.dirs()
	if err != nil {
		t.Fatal(err)
	}
	if record, found, err := policy.ReadRecord(dirs.State, root); err != nil || !found || record.PolicyDigest != pt.Digest {
		t.Fatalf("the refused record = %+v, %v, %v", record, found, err)
	}
}

// TestAgentInstallNamesAReenrollment prints the digest the record held and
// the one agent install enrolls, before the diff and the write.
func TestAgentInstallNamesAReenrollment(t *testing.T) {
	t.Parallel()
	ta, port, root := agentApp(t)
	port.mismatch = "sha256:old"
	if got := ta.run(t.Context(), []string{"agent", "install", "--agent", agents.IDs()[0], root}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	out := ta.stdout.String()
	line := "enrollment: policy sha256:old becomes sha256:policy\n"
	if !strings.HasPrefix(out, line) || port.calls != 1 {
		t.Fatalf("stdout = %q, port = %+v", out, port)
	}
	broken, brokenPort, root := agentApp(t)
	brokenPort.readErr = errBoom
	if got := broken.run(t.Context(), []string{"agent", "install", "--agent", agents.IDs()[0], root}); got != exitFailure || brokenPort.calls != 0 {
		t.Fatalf("exit = %d, stderr = %q, port = %+v", got, broken.stderr.String(), brokenPort)
	}
}

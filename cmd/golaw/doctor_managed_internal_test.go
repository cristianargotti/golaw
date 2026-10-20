package main

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
)

func TestDoctorNamesTheDriftOfAManagedFile(t *testing.T) {
	t.Parallel()
	root := repository(t, false)
	managed := "managed:\n  - path: golaw.mk\n    sha256: " + install.Digest([]byte("recorded\n")) + "\n"
	writeFiles(t, root, map[string]string{
		"golaw.yml": strings.Replace(string(renderedPolicy(t)), "managed: []\n", managed, 1),
		"golaw.mk":  "edited by hand\n",
	})
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "build: adopt the golaw policy")
	ta, _ := doctorApp(t)
	if code := ta.run(t.Context(), []string{"doctor", root}); code != exitClean {
		t.Fatalf("exit = %d, stderr = %q", code, ta.stderr.String())
	}
	drift := "doctor.managed golaw.mk: drift [managed-drift] golaw.mk differs from the hash init recorded: golaw init --update shows the diff and keeps the edit\n"
	if got := strings.Count(ta.stdout.String(), drift); got != 1 {
		t.Fatalf("drift lines = %d in %q", got, ta.stdout.String())
	}
	writeFiles(t, root, map[string]string{"golaw.mk": "recorded\n"})
	ta.stdout.Reset()
	if code := ta.run(t.Context(), []string{"doctor", root}); code != exitClean || !strings.Contains(ta.stdout.String(), "doctor.managed golaw.mk: ok [managed-matches] golaw.mk\n") {
		t.Fatalf("a restored file = %d, %q", code, ta.stdout.String())
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// floorText is the fix a refused capability set names.
const floorText = "capability api needs go: add go to the capabilities"

// capabilityRepository is a repository whose committed policy enables the
// capabilities of the line given in place of the rendered [go].
func capabilityRepository(t *testing.T, line string) string {
	t.Helper()
	root := repository(t, false)
	text := strings.Replace(string(renderedPolicy(t)), "capabilities: [go]\n", line, 1)
	if err := os.WriteFile(filepath.Join(root, "golaw.yml"), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "build: the policy")
	return root
}

func TestScanRefusesAGoShapeWithoutGoBeforeAnyCheck(t *testing.T) {
	t.Parallel()
	ta := boundApp(t)
	root := capabilityRepository(t, "capabilities: [api]\n")
	if got := ta.run(t.Context(), []string{"scan", root}); got != exitFailure {
		t.Fatalf("exit = %d, stdout = %q", got, ta.stdout.String())
	}
	if !strings.Contains(ta.stderr.String(), floorText) || ta.stdout.Len() != 0 {
		t.Fatalf("stdout = %q, stderr = %q", ta.stdout.String(), ta.stderr.String())
	}
	near := boundApp(t)
	nearRoot := capabilityRepository(t, "capabilities: [api, go]\n")
	coreAssets(t, nearRoot)
	if got := near.run(t.Context(), []string{"scan", nearRoot}); got != exitClean {
		t.Fatalf("api with go: exit = %d, stderr = %q", got, near.stderr.String())
	}
}

func TestInitRefusesAGoShapeOutsideAGoModule(t *testing.T) {
	t.Parallel()
	ta, record := initApp(t)
	root := checkout(t)
	if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(err)
	}
	if got := ta.run(t.Context(), []string{"init", "--apply", "--capabilities", "api", root}); got != exitFailure {
		t.Fatalf("exit = %d", got)
	}
	if !strings.Contains(ta.stderr.String(), floorText) || record.calls != 0 {
		t.Fatalf("stderr = %q, enrollment = %+v", ta.stderr.String(), record)
	}
	if _, err := os.Stat(filepath.Join(root, "golaw.yml")); !os.IsNotExist(err) {
		t.Fatalf("a refused set wrote the policy: %v", err)
	}
}

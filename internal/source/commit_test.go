package source_test

import "testing"

// commitAll stages and commits everything under root and returns the
// commit id.
func commitAll(t *testing.T, root, message string) string {
	t.Helper()
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", message)
	return gitRun(t, root, "rev-parse", "HEAD")
}

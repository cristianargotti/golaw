package codex_test

import (
	"bytes"
	"reflect"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/codex"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestRouteCatalogAndAnswers(t *testing.T) {
	t.Parallel()
	manifest, err := proof.LoadManifest("proof/manifest.yml")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"codex.envelope",
		"codex.apply-patch-add",
		"codex.apply-patch-update",
		"codex.apply-patch-unapplied",
		"codex.apply-patch-move",
		"codex.apply-patch-delete",
		"codex.bash",
		"codex.heredoc-patch",
		"codex.heredoc-patch-cd",
		"codex.heredoc-cat",
		"codex.post-apply-patch",
		"codex.post-bash",
	}
	got := manifest.Names()
	slices.Sort(got)
	slices.Sort(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("route catalog = %v, want %v", got, want)
	}
	for _, entry := range manifest.Checks {
		for _, path := range []string{entry.Red, entry.Near} {
			directory := path[len("testdata/"):]
			event, result := routeResult(t, directory)
			reply := codex.Encode(event, result)
			assertRouteAnswer(t, directory, result, reply)
		}
	}
}

// assertRouteAnswer holds the answer of a route fixture to its outcome: a
// denial blocks, a finding reaches the agent and a near miss is allowed.
func assertRouteAnswer(t *testing.T, directory string, result hook.Result, reply hook.Reply) {
	t.Helper()
	if result.Outcome == hook.Deny &&
		(reply.ExitCode != 0 || reply.Stderr == "" ||
			!bytes.Contains(reply.Stdout, []byte("\"permissionDecision\":\"deny\""))) {
		t.Fatalf("%s denial lost its blocking answer", directory)
	}
	if result.Outcome == hook.Feedback &&
		(reply.ExitCode != 0 || len(reply.Stdout) == 0) {
		t.Fatalf("%s feedback lost its answer", directory)
	}
	if result.Outcome == hook.Allow && reply.ExitCode != 0 {
		t.Fatalf("%s near miss was refused", directory)
	}
}

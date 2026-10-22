package hook_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// heredocRequest is a shell pre event whose text writes one file, as the
// decoder of a here-document reconstructed it.
func heredocRequest(path, content string) hook.Request {
	req := hostShell("Bash")
	req.Event.Agent = "codex"
	req.Writes = &hook.Writes{
		Candidates: []model.Candidate{{Path: path, Op: model.CandidateCreate, Reconstructed: true}},
		Content:    map[string][]byte{path: []byte(content)},
	}
	return req
}

func TestShellWritesAreJudgedBeforeTheCommand(t *testing.T) {
	t.Parallel()
	rows := []struct {
		name, path, content, reason string
		want                        hook.Outcome
	}{
		{"red text", "notes.go", "bad", "text.probe: invalid text (notes.go)", hook.Deny},
		{"red policy file", "golaw.yml", "safe", "golaw.yml", hook.Deny},
		{"near safe", "notes.go", "safe", "", hook.Allow},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			adapter, dependencies := harness(t, heredocRequest(row.path, row.content))
			event, got := hook.Evaluate(t.Context(), "codex", []byte("{}"), adapter, dependencies)
			if got.Outcome != row.want || !strings.Contains(got.Reason, row.reason) || event.Shape != model.ShapeShell {
				t.Fatalf("%+v %s", got, event.Shape)
			}
			key, err := hook.StateKey(event.Cwd, event)
			if err != nil {
				t.Fatal(err)
			}
			_, loadErr := dependencies.State.Load(t.Context(), key)
			if (loadErr == nil) != (row.want == hook.Allow) {
				t.Fatalf("baseline kept = %v for %s", loadErr == nil, row.want)
			}
		})
	}
}

func TestUnappliedCandidateIsNamedInTheNotice(t *testing.T) {
	t.Parallel()
	rows := []struct {
		name          string
		reconstructed bool
		content       string
		want          hook.Outcome
		notice        bool
	}{
		{"red added lines judged", false, "bad", hook.Deny, false},
		{"red unapplied named", false, "safe", hook.Allow, true},
		{"near whole candidate", true, "safe", hook.Allow, false},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			req := request(row.content)
			req.Event.Candidates[0].Op, req.Event.Candidates[0].Reconstructed = model.CandidateEdit, row.reconstructed
			adapter, dependencies := harness(t, req)
			_, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
			named := strings.Contains(got.Notice, "a.go") && strings.Contains(got.Notice, "added lines")
			if got.Outcome != row.want || named != row.notice {
				t.Fatalf("%+v", got)
			}
		})
	}
}

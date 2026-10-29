package cursor_test

import (
	"encoding/json"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/cursor"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestEncodeGivesTheLinesAsAdditionalContext(t *testing.T) {
	t.Parallel()
	lines := []string{"scripts/smoke.sh:4 shellcheck/SC2086 Double quote to prevent globbing and word splitting."}
	post := model.Event{Shape: model.ShapePost, Post: &model.PostResult{}}
	for name, tc := range map[string]struct {
		result hook.Result
		want   string
	}{
		"an allow":       {hook.Result{Outcome: hook.Allow, Notice: "golaw did not measure a.py.", Lines: lines}, "golaw did not measure a.py.\n" + lines[0]},
		"lines alone":    {hook.Result{Outcome: hook.Allow, Lines: lines}, lines[0]},
		"a post finding": {hook.Result{Outcome: hook.Feedback, Reason: "goast.x: y (a.go)", Lines: lines}, "goast.x: y (a.go)\n" + lines[0]},
	} {
		reply := cursor.Encode(post, tc.result)
		var document struct {
			Context    string `json:"additional_context"`
			Permission string `json:"permission"`
		}
		if reply.ExitCode != 0 || json.Unmarshal(reply.Stdout, &document) != nil || document.Context != tc.want || document.Permission != "" {
			t.Errorf("%s = %+v, %+v", name, reply, document)
		}
	}
}

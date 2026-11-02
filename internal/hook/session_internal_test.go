package hook

import (
	"context"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

// TestSessionAllowsWithOrWithoutTheMap pins that a session start is an
// allow whatever its binding answers, so it never stops a session.
func TestSessionAllowsWithOrWithoutTheMap(t *testing.T) {
	t.Parallel()
	run := execution{repository: Repository{Root: "root"}}
	if got := run.session(t.Context(), Dependencies{}); got.Outcome != Allow || got.Notice != "" {
		t.Fatalf("unbound = %+v", got)
	}
	var seen string
	deps := Dependencies{Session: func(_ context.Context, repository Repository, _ *model.EffectivePolicy) string {
		seen = repository.Root
		return "map"
	}}
	if got := run.session(t.Context(), deps); got.Outcome != Allow || got.Notice != "map" || seen != "root" {
		t.Fatalf("bound = %+v, root %q", got, seen)
	}
}

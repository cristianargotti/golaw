package hook_test

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// targetsAdapter is the harness adapter whose parse also names the
// further targets of a write of several files.
func targetsAdapter(req hook.Request, targets []string) hook.Adapter {
	adapter := harnessAdapter(req)
	parse := adapter.Parse
	adapter.Parse = func(data []byte) (hook.Parsed, error) {
		parsed, err := parse(data)
		parsed.Targets = targets
		return parsed, err
	}
	return adapter
}

// TestEnrollmentReadsEveryTargetOfAWrite looks enrollment up at the target
// of a write and then at each further target its parse names, in order,
// so a write of several files whose first file lies outside every
// repository never lends that location to a file inside one (A3), and a
// write whose every target is unenrolled keeps the valid allow.
func TestEnrollmentReadsEveryTargetOfAWrite(t *testing.T) {
	t.Parallel()
	repo, outside := realDir(t), realDir(t)
	for _, tc := range []struct {
		name    string
		targets []string
		want    []string
		outcome hook.Outcome
	}{
		{"red a further target in the repository", []string{filepath.Join(repo, "b.md")}, []string{outside, repo}, hook.Deny},
		{"red a move destination after another outside", []string{filepath.Join(outside, "c.md"), filepath.Join(repo, "moved.md")}, []string{outside, repo}, hook.Deny},
		{"near every target outside", []string{filepath.Join(outside, "c.md")}, []string{outside}, hook.Allow},
		{"near one target", nil, []string{outside}, hook.Allow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := request("bad")
			req.Event.Cwd, req.Event.Target = outside, filepath.Join(outside, "a.md")
			dependencies := harnessDependencies(t, repo)
			enrolled := dependencies.Enroll
			var calls []string
			dependencies.Enroll = func(ctx context.Context, dir, cwd string) (hook.Repository, error) {
				calls = append(calls, dir)
				if dir == repo {
					return enrolled(ctx, dir, cwd)
				}
				return hook.Repository{Enrollment: model.Enrollment{State: model.EnrollmentNone}}, nil
			}
			_, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), targetsAdapter(req, tc.targets), dependencies)
			if got.Outcome != tc.outcome || !slices.Equal(calls, tc.want) {
				t.Fatalf("outcome %s, enrolled from %v; want %s from %v", got.Outcome, calls, tc.outcome, tc.want)
			}
		})
	}
}

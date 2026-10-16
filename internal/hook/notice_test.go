package hook_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// noticing encodes the outcome and the notice of a result, so a test reads
// what the agent would be told beside the verdict.
func noticing(adapter hook.Adapter) hook.Adapter {
	adapter.Encode = func(_ model.Event, result hook.Result) hook.Reply {
		code := 0
		if result.CannotJudge {
			code = 2
		}
		return hook.Reply{Stdout: []byte(string(result.Outcome) + "|" + result.Notice), Stderr: result.Reason, ExitCode: code}
	}
	return adapter
}

// largeTree is a snapshot of a safe source file and a text file over the
// read bound, whose size it knows and whose bytes it never holds, as the
// worktree answers such a file.
type largeTree struct{ size int64 }

func (l largeTree) Files() []model.File {
	return []model.File{{Path: "a.go", Class: model.ClassTracked, Size: 4}, {Path: "big.sql", Class: model.ClassTracked, Size: l.size}}
}

func (largeTree) Class(path string) model.FileClass {
	if path == "a.go" || path == "big.sql" {
		return model.ClassTracked
	}
	return model.ClassUnknown
}

func (largeTree) Read(path string) ([]byte, error) {
	if path == "a.go" {
		return []byte("safe"), nil
	}
	return nil, model.ErrTooLarge
}

// TestAChangedFileOverTheReadBoundIsANotice identifies a file over the
// read bound by its size before and after a command, and answers its
// change with an allow whose notice and human message name the file and
// leave it to the pipeline, instead of a detection that blocks.
func TestAChangedFileOverTheReadBoundIsANotice(t *testing.T) {
	t.Parallel()
	req := shellRequest()
	adapter, dependencies := harness(t, req)
	dependencies.Snapshot = func(context.Context, hook.Repository) (model.Snapshot, error) {
		return largeTree{size: model.MaxReadBytes + 1}, nil
	}
	event, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if got.Outcome != hook.Allow || got.Notice != "" {
		t.Fatal(got)
	}
	adapter = asPostEvent(&req, event.Cwd, false)
	_, got = hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if got.Outcome != hook.Allow || got.Notice != "" {
		t.Fatalf("an unchanged file over the bound was named: %+v", got)
	}
	dependencies.Snapshot = func(context.Context, hook.Repository) (model.Snapshot, error) {
		return largeTree{size: model.MaxReadBytes + 2}, nil
	}
	_, got = hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
	if got.Outcome != hook.Allow || got.CannotJudge || !strings.Contains(got.Notice, "big.sql") ||
		!strings.Contains(got.Notice, "the pipeline judges") || !strings.Contains(got.Human, "big.sql") {
		t.Fatal(got)
	}
}

// TestThePostDeadlineIsANotice answers a post event the deadline overtook
// with an allow and a notice, while a pre event the deadline overtakes
// stays a denial.
func TestThePostDeadlineIsANotice(t *testing.T) {
	t.Parallel()
	req := shellRequest()
	_, dependencies := harness(t, req)
	adapter := noticing(asPostEvent(&req, req.Event.Cwd, false))
	dependencies.Budget = time.Millisecond
	dependencies.Enroll = func(ctx context.Context, _, _ string) (hook.Repository, error) {
		<-ctx.Done()
		return hook.Repository{}, ctx.Err()
	}
	reply := hook.Run(t.Context(), "claude", strings.NewReader("{}"), adapter, dependencies)
	if reply.ExitCode != 0 || !strings.HasPrefix(string(reply.Stdout), "allow|") || !strings.Contains(string(reply.Stdout), "deadline") {
		t.Fatal(reply)
	}
	pre, deps := harness(t, request("safe"))
	deps.Budget, deps.Enroll = time.Millisecond, dependencies.Enroll
	if reply = hook.Run(t.Context(), "claude", strings.NewReader("{}"), pre, deps); reply.ExitCode != 2 || !strings.Contains(reply.Stderr, "deadline") {
		t.Fatal(reply)
	}
}

// boundCase is one event over the input bound: the header the adapter
// reads, the enrollment and the policy it meets, and the answer it wants.
type boundCase struct {
	name, target, want string
	shape              model.EventShape
	header             bool
	state              model.EnrollmentState
	policy, enroll     error
}

// TestAnInputOverTheBoundIsMeasuredWhereItCan answers an event whose input
// is over the bound from its header: a post event and a write leave a
// notice naming what was not measured, a write of golaw.yml stays a
// denial, an unenrolled location is a plain allow, a missing policy a
// denial, and any other event, a write leading out of its repository, a
// failed enrollment or an adapter that reads no header stay cannot judge.
func TestAnInputOverTheBoundIsMeasuredWhereItCan(t *testing.T) {
	t.Parallel()
	input := `{"x":"` + strings.Repeat("a", model.MaxReadBytes) + `"}`
	enrolled := model.EnrollmentEnrolled
	for _, tc := range []boundCase{
		{"a post event", "", "allow|", model.ShapePost, true, enrolled, nil, nil},
		{"a write", "notes.md", "allow|", model.ShapeWrite, true, enrolled, nil, nil},
		{"a write to a name the rules forbid", "forbidden/notes.md", "deny|", model.ShapeWrite, true, enrolled, nil, nil},
		{"a write of the policy", "golaw.yml", "deny|", model.ShapeWrite, true, enrolled, nil, nil},
		{"an unenrolled location", "notes.md", "plain", model.ShapeWrite, true, model.EnrollmentNone, nil, nil},
		{"a missing policy", "notes.md", "deny|", model.ShapeWrite, true, enrolled, model.ErrEnrollment, nil},
		{"a write out of its repository", "../outside.md", "", model.ShapeWrite, true, enrolled, nil, nil},
		{"a failed enrollment", "notes.md", "", model.ShapeWrite, true, enrolled, nil, model.ErrCannotJudge},
		{"a shell event", "", "", model.ShapeShell, true, enrolled, nil, nil},
		{"an adapter without a header reader", "notes.md", "", model.ShapeWrite, false, enrolled, nil, nil},
	} {
		adapter, dependencies := boundHarness(t, tc)
		if reply := hook.Run(t.Context(), "claude", strings.NewReader(input), adapter, dependencies); !answers(reply, tc.want, tc.target) {
			t.Fatalf("%s: %+v", tc.name, reply)
		}
	}
}

// boundHarness is the harness of a bound case over a real directory.
func boundHarness(t *testing.T, tc boundCase) (hook.Adapter, hook.Dependencies) {
	t.Helper()
	adapter, dependencies := harness(t, request("safe"))
	adapter = noticing(adapter)
	root := realDir(t)
	dependencies.Enroll = func(context.Context, string, string) (hook.Repository, error) {
		return hook.Repository{Root: root, Identity: root, Enrollment: model.Enrollment{State: tc.state}}, tc.enroll
	}
	if tc.policy != nil {
		dependencies.Policy = func(context.Context, hook.Repository) (*model.EffectivePolicy, error) { return nil, tc.policy }
	}
	if tc.header {
		adapter.Oversized = func([]byte) (model.Event, error) {
			event := model.Event{Agent: "claude", Tool: "Write", Shape: tc.shape, Cwd: root, Target: tc.target}
			if tc.shape == model.ShapePost {
				event.Post = &model.PostResult{}
			}
			return event, nil
		}
	}
	return adapter, dependencies
}

// answers reports whether a reply is the one a bound case wants: cannot
// judge when it wants none, an allow with no notice for plain, else its
// outcome, with a notice of the bound that names the target for an allow.
func answers(reply hook.Reply, want, target string) bool {
	said := string(reply.Stdout)
	switch want {
	case "":
		return reply.ExitCode == 2
	case "plain":
		return reply.ExitCode == 0 && said == "allow|"
	case "allow|":
		return reply.ExitCode == 0 && strings.HasPrefix(said, want) && strings.Contains(said, "8 MiB") && strings.Contains(said, target)
	default:
		return reply.ExitCode == 0 && strings.HasPrefix(said, want)
	}
}

// TestADecoderGapIsANotice answers a write the decoder could not
// reconstruct within the read bound by the name rules over its file, whose
// bytes on disk are over the bound: a name they forbid is denied, any
// other is an allow with a notice naming the file, and golaw.yml stays
// protected however large it is.
func TestADecoderGapIsANotice(t *testing.T) {
	t.Parallel()
	for path, want := range map[string]hook.Outcome{"big.sql": hook.Allow, "forbidden/big.sql": hook.Deny, "golaw.yml": hook.Deny} {
		req := request("safe")
		req.Event.Candidates[0].Path, req.Content = path, nil
		req.Gaps = []hook.Gap{hook.OverReadBound(path)}
		adapter, dependencies := harness(t, req)
		large := proof.NewTree(map[string][]byte{path: make([]byte, model.MaxReadBytes+1)})
		dependencies.Snapshot = func(context.Context, hook.Repository) (model.Snapshot, error) { return large, nil }
		_, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
		if got.Outcome != want || want == hook.Allow && (!strings.Contains(got.Notice, path) || !strings.Contains(got.Human, path)) {
			t.Fatalf("%s: %+v", path, got)
		}
	}
}

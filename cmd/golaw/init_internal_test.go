package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/buildinfo"
	"github.com/cristianargotti/golaw/internal/model"
)

// enrollment is what the fake enrollment seam recorded.
type enrollment struct {
	root   string
	digest string
	calls  int
}

// initApp wires the init seams with fakes: compile answers the core
// defaults with a digest derived from the bytes, enroll records its call.
func initApp(t *testing.T) (testApp, *enrollment) {
	t.Helper()
	ta := newTestApp(t)
	record := &enrollment{}
	ta.compile = func(data []byte, _ time.Time) (*model.EffectivePolicy, error) {
		policy := model.CoreDefaults()
		policy.Digest = "sha256:" + strings.Repeat("a", len(data)%7)
		policy.Enrollment.State = model.EnrollmentEnrolled
		return &policy, nil
	}
	ta.enroll = func(root string, _ []byte, policy *model.EffectivePolicy, _ time.Time) error {
		record.root, record.digest, record.calls = root, policy.Digest, record.calls+1
		return nil
	}
	return ta, record
}

// checkout creates a git checkout root with a go.mod and returns its real
// path, so the symbolic link of the temporary directory on macOS never
// enters a comparison.
func checkout(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/m\n\ngo 1.27\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

// twoDecisionDirectories is a checkout with both docs/adr and docs/decisions.
func twoDecisionDirectories(t *testing.T) string {
	t.Helper()
	root := checkout(t)
	for _, dir := range []string{"docs/adr", "docs/decisions"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestInitWithoutApplyPrintsTheDiffAndWritesNothing(t *testing.T) {
	t.Parallel()
	ta, record := initApp(t)
	root := checkout(t)
	if got := ta.run(t.Context(), []string{"init", root}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	out := ta.stdout.String()
	wanted := []string{
		"--- /dev/null\n", "+golaw: 1\n", "+kit: { version: 9.9.9 }\n", "+language: pt-BR\n",
		"+capabilities: [go]\n", "init: nothing written, run again with --apply\n",
	}
	for _, want := range wanted {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(root, bootstrap.PolicyFile)); !os.IsNotExist(err) {
		t.Fatalf("policy file written without --apply: %v", err)
	}
	if record.calls != 0 {
		t.Fatalf("enrolled without --apply: %+v", record)
	}
}

func TestInitApplyWritesEnrollsAndIsIdempotent(t *testing.T) {
	t.Parallel()
	ta, record := initApp(t)
	root := checkout(t)
	args := []string{"init", "--apply", "--language", "en-US", "--capabilities", "db,api", root}
	if got := ta.run(t.Context(), args); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(root, bootstrap.PolicyFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"language: en-US\n", "capabilities: [api, db, go]\n", "layout: { main: root, decisions: docs/adr }\n"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("missing %q in:\n%s", want, data)
		}
	}
	if record.calls != 1 || record.root != root || !strings.HasPrefix(record.digest, "sha256:") {
		t.Fatalf("enrollment = %+v", record)
	}
	if !strings.HasSuffix(ta.stdout.String(), "enrolled: "+root+" policy "+record.digest+"\n") {
		t.Fatalf("stdout = %q", ta.stdout.String())
	}
	ta.stdout.Reset()
	if got := ta.run(t.Context(), args); got != exitClean {
		t.Fatalf("second exit = %d, stderr = %q", got, ta.stderr.String())
	}
	if !strings.HasPrefix(ta.stdout.String(), "golaw.yml: no changes\n") || record.calls != 2 {
		t.Fatalf("second run stdout = %q, enrollment = %+v", ta.stdout.String(), record)
	}
}

func TestInitPinsTheDevelopmentVersionOfAnUntaggedBuild(t *testing.T) {
	t.Parallel()
	ta, _ := initApp(t)
	ta.buildInfo = func() buildinfo.Info {
		return buildinfo.Info{Version: "0.0.0-20261015004959-dd849024b474", Stamped: true}
	}
	if got := ta.run(t.Context(), []string{"init", checkout(t)}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	if !strings.Contains(ta.stdout.String(), "version: "+developmentPin) {
		t.Fatalf("stdout = %q", ta.stdout.String())
	}
}

func TestInitRefusals(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		args func(t *testing.T) []string
		want string
	}{
		"outside git": {func(t *testing.T) []string {
			t.Helper()
			return []string{"init", t.TempDir()}
		}, "not a git repository root"},
		"two decision directories": {func(t *testing.T) []string {
			t.Helper()
			return []string{"init", twoDecisionDirectories(t)}
		}, "both docs/adr and docs/decisions exist"},
		"unknown capability": {func(t *testing.T) []string {
			t.Helper()
			return []string{"init", "--capabilities", "web", checkout(t)}
		}, `unknown capability "web"`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ta, record := initApp(t)
			if got := ta.run(t.Context(), tc.args(t)); got != exitFailure {
				t.Fatalf("exit = %d", got)
			}
			stderr := ta.stderr.String()
			if !strings.HasPrefix(stderr, "golaw: init: ") || !strings.Contains(stderr, tc.want) || record.calls != 0 {
				t.Fatalf("stderr = %q, enrollment = %+v", stderr, record)
			}
		})
	}
}

func TestInitDoesNotWriteAPolicyTheKitCannotCompile(t *testing.T) {
	t.Parallel()
	ta, record := initApp(t)
	ta.compile = func([]byte, time.Time) (*model.EffectivePolicy, error) { return nil, errBoom }
	root := checkout(t)
	if got := ta.run(t.Context(), []string{"init", "--apply", root}); got != exitFailure {
		t.Fatalf("exit = %d", got)
	}
	if _, err := os.Stat(filepath.Join(root, bootstrap.PolicyFile)); !os.IsNotExist(err) {
		t.Fatalf("policy written after a compile failure: %v", err)
	}
	if ta.stderr.String() != "golaw: init: boom\n" || record.calls != 0 {
		t.Fatalf("stderr = %q, enrollment = %+v", ta.stderr.String(), record)
	}
}

package genguard_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/genguard"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// shellRun judges a shell pre event over a worktree tree, its root and
// working directory spelled the way the hook core sets them.
func shellRun(t *testing.T, c model.Check, files map[string]string, command string) model.Result {
	t.Helper()
	tree := map[string][]byte{}
	for p, text := range files {
		tree[p] = []byte(text)
	}
	event := &model.Event{Shape: model.ShapeShell, Agent: "tester", Tool: "Bash", Shell: &model.ShellCommand{Command: command, Cwd: "/project/internal/application", Tool: "Bash", Root: "/project"}}
	res, err := c.Run(t.Context(), model.Input{Mode: model.ModeHook, Policy: proof.Policy(), Snapshot: proof.NewTree(tree), Event: event})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestTheGenDenialsJudgeTheLiteralTargetsOfAShellCommand(t *testing.T) {
	t.Parallel()
	files := map[string]string{manifest.Path: manifestYAML(t), fakePath: golawHeader}
	tests := map[string]struct {
		check   model.Check
		command string
		path    string
	}{
		"red: a redirection to a generated path":       {genguard.GeneratedWrite(), "cat > growth/ledgertest/zz_generated_fake.go <<'EOF'\npackage ledgertest\nEOF\n", fakePath},
		"red: sed in place over a generated path":      {genguard.GeneratedWrite(), "sed -i 's/a/b/' growth/ledgertest/zz_generated_fake.go", fakePath},
		"red: rm of the manifest":                      {genguard.ManifestWrite(), "rm ../../.golaw/gen.yml", manifest.Path},
		"near: a redirection to a variable":            {genguard.GeneratedWrite(), "cat > \"$OUT\" <<'EOF'\npackage ledgertest\nEOF\n", ""},
		"near: a redirection to an owned file":         {genguard.GeneratedWrite(), "echo x >> growth/ledgertest/cases.go", ""},
		"near: a path outside the repository":          {genguard.GeneratedWrite(), "echo x > /elsewhere/zz_generated_fake.go", ""},
		"near: the command that regenerates the files": {genguard.GeneratedWrite(), "golaw gen", ""},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			res := shellRun(t, tc.check, files, tc.command)
			if tc.path == "" {
				if len(res.Findings) != 0 {
					t.Fatalf("findings = %+v", res.Findings)
				}
				return
			}
			if len(res.Findings) != 1 || res.Findings[0].Path != tc.path {
				t.Fatalf("findings = %+v, want one at %s", res.Findings, tc.path)
			}
		})
	}
}

func TestAShellTargetOverTheReadBoundAddsNoNotice(t *testing.T) {
	t.Parallel()
	const big = "big.sql"
	tree := failingTree{Tree: proof.NewTree(map[string][]byte{big: nil}), fail: map[string]error{big: model.ErrTooLarge}}
	event := &model.Event{Shape: model.ShapeShell, Agent: "tester", Tool: "Bash", Shell: &model.ShellCommand{Command: "printf x >> big.sql", Cwd: "/project", Tool: "Bash", Root: "/project"}}
	res, err := genguard.GeneratedWrite().Run(t.Context(), model.Input{Mode: model.ModeHook, Policy: proof.Policy(), Snapshot: tree, Event: event})
	if err != nil || len(res.Unread) != 0 || len(res.Findings) != 0 {
		t.Fatalf("res = %+v, %v", res, err)
	}
}

package shell_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/shell"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// input builds a hook input for one shell command over a tree.
func input(command string, policy *model.EffectivePolicy, snap model.Snapshot) model.Input {
	event := &model.Event{Shape: model.ShapeShell, Agent: "tester", Tool: "Bash", Shell: &model.ShellCommand{Command: command, Cwd: "/work", Tool: "Bash"}}
	return model.Input{Mode: model.ModeHook, Policy: policy, Snapshot: snap, Event: event}
}

// judge runs one check over a command under the proof policy and an empty
// tree.
func judge(t *testing.T, c model.Check, command string) model.Result {
	t.Helper()
	res, err := c.Run(t.Context(), input(command, proof.Policy(), proof.NewTree(nil)))
	if err != nil {
		t.Fatal(err)
	}
	if res.Status.Kind != model.StatusMeasured || res.Scope != "1 command" {
		t.Fatalf("%q: status %+v, scope %q", command, res.Status, res.Scope)
	}
	return res
}

// codes lists the finding codes of a result in order.
func codes(res model.Result) []string {
	out := make([]string, 0, len(res.Findings))
	for _, f := range res.Findings {
		out = append(out, f.Code)
	}
	return out
}

// denies runs a table of commands and checks which ones yield the code.
func denies(t *testing.T, c model.Check, code string, cases map[string]bool) {
	t.Helper()
	for command, want := range cases {
		got := codes(judge(t, c, command))
		if (len(got) == 1 && got[0] == code) != want || len(got) > 1 {
			t.Errorf("%q: codes %v, want finding %v", command, got, want)
		}
	}
}

func TestChecksAreValidAndJudgeShellEventsOnly(t *testing.T) {
	t.Parallel()
	read := &model.Event{Shape: model.ShapeRead, Agent: "tester", Tool: "Read", Read: &model.ReadRequest{Path: "x"}}
	for _, c := range shell.Checks() {
		s := c.Spec()
		if err := s.Validate(); err != nil {
			t.Errorf("%s: %v", s.ID, err)
		}
		if s.Proof != s.ID || s.Scope != model.ScopeEvent || s.Owner != model.OwnerCore {
			t.Errorf("%s: spec = %+v", s.ID, s)
		}
		res, err := c.Run(t.Context(), model.Input{Mode: model.ModeHook, Policy: proof.Policy(), Snapshot: proof.NewTree(nil), Event: read})
		if err != nil || res.Status.Kind != model.StatusNotApplicable {
			t.Errorf("%s on a read event: %+v, %v", s.ID, res, err)
		}
	}
}

func TestNoVerify(t *testing.T) {
	t.Parallel()
	denies(t, shell.NoVerify(), "no-verify", map[string]bool{
		"git commit --no-verify -m x":                   true,
		"git commit -n -m x":                            true,
		"git commit -m x":                               false,
		"git merge --no-verify topic":                   true,
		"git merge -n topic":                            false,
		"git push --no-verify origin main":              true,
		"git -c core.hooksPath=/dev/null commit -m x":   true,
		"git -C sub commit --no-verify -m x":            true,
		"bash -lc 'git commit --no-verify -m x'":        true,
		`pwsh -NoProfile -Command "git commit -n -m x"`: true,
		"cmd /c git commit --no-verify -m x":            true,
		"git commit -m 'docs: explain --no-verify'":     false,
		"echo git commit --no-verify":                   false,
		"FOO=1 timeout 30 git commit --no-verify -m x":  true,
		"sudo git commit --no-verify -m x":              true,
		"git status; git commit -m x && git log -n 1":   false,
		"git.exe commit --no-verify -m x":               true,
		"C:/Git/bin/git.exe commit -n -m x":             true,
		"git.exe commit -m x":                           false,
		"git.exe status":                                false,
	})
}

func TestTokenizerShapes(t *testing.T) {
	t.Parallel()
	denies(t, shell.NoVerify(), "no-verify", map[string]bool{
		"git commit \\\n  --no-verify -m x":             true,
		"git commit -m fix:\\ x --no-verify":            true,
		`git commit -m "a \"quoted\" word" --no-verify`: true,
		"git commit -m 'it''s' --no-verify":             true,
		"(git commit --no-verify -m x)":                 true,
		"git":                                           false,
		"git -C":                                        false,
		"FOO=1":                                         false,
		"bash":                                          false,
		"bash script.sh":                                false,
		"timeout":                                       false,
		"git commit -m 'x; git commit --no-verify'": false,
		"echo $(git commit --no-verify -m x)":       true,
	})
}

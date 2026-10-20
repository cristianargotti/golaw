package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
)

// m2Step is what a lifecycle answer knows of one command: the row, its
// family, the command's kind, whether the mode corrupts and how many
// commands of each kind the row ran.
type m2Step struct {
	name    string
	family  string
	kind    string
	corrupt bool
	count   map[string]int
}

// m2LifecycleScript answers a lifecycle row: the candidate's init, setup,
// uninstall, update, doctor and rung commands, the tools' version probes,
// the interrupt controller and the commit through the hooks, each by the
// row's family and the mode, never by running the candidate or a tool.
func m2LifecycleScript(t *testing.T, s *suite, name, mode string) *m2Script {
	t.Helper()
	return newM2Script(t, s, m2LifecycleAnswer(t, s, name, mode))
}

// m2LifecycleAnswer is the answer of one lifecycle row; broken fails the
// row's target operation with an execution error.
func m2LifecycleAnswer(t *testing.T, s *suite, name, mode string) m2Answer {
	t.Helper()
	step := m2Step{name: name, family: m2Family(name), corrupt: mode == "corrupt", count: map[string]int{}}
	return func(ctx context.Context, c command) (m2Reply, bool) {
		step.kind = m2Kind(s, c)
		target := m2Target(ctx, t, s, step, c)
		if mode == "broken" && target {
			return m2Reply{got: result{err: ErrAcceptance}, target: true}, true
		}
		step.count[step.kind]++
		got, handled := m2Lifecycle(ctx, t, s, step, c)
		return m2Reply{got: got, target: target}, handled
	}
}

// m2Lifecycle answers a command of a lifecycle row by its kind; a plain
// commit and every other Git command go to the fixture executor.
func m2Lifecycle(ctx context.Context, t *testing.T, s *suite, step m2Step, c command) (result, bool) {
	t.Helper()
	switch step.kind {
	case "init":
		got := m2Init(t, s, c)
		m2CorruptInit(ctx, t, s, step, c)
		return got, true
	case "setup":
		alter := step.corrupt && step.family == "setup-idempotent" && step.count["setup"] > 1
		return m2Setup(ctx, t, s, c, alter), true
	case "uninstall":
		return m2Uninstall(t, s, c, step.corrupt), true
	case "update":
		return m2Update(t, s, c, step.corrupt), true
	case "doctor":
		return m2Doctor(t, s, c), true
	case "hook":
		return m2Rung(step.name, step.corrupt), true
	case "version":
		return result{stdout: "golaw " + m2Version + "\nbuild: source build, executable not verifiable\n"}, true
	case "controller":
		return m2Controller(ctx, t, s, c, step.corrupt), true
	case "probe":
		return m2ProbeResult(c, step.corrupt && step.family == "setup-clean"), true
	case "commit":
		if !m2Armed(ctx, t, s, c) {
			return result{}, false
		}
		return m2Through(ctx, t, s, c, step), true
	}
	return result{}, false
}

// m2Family names the family of a lifecycle row: the switch rows together,
// the older and newer pins together as refused, the equal pins together,
// and every other row by itself.
func m2Family(name string) string {
	switch {
	case strings.HasSuffix(name, "-disabled"):
		return "disabled"
	case strings.HasSuffix(name, "-older"), strings.HasSuffix(name, "-newer"):
		return "refused"
	case strings.HasSuffix(name, "-equal"):
		return "equal"
	}
	return name
}

// m2Kind names what a command does: a Git subcommand, a command of the
// candidate, the interrupt controller or the version probe of a tool.
func m2Kind(s *suite, c command) string {
	switch {
	case c.program == s.git && m2Subcommand(c.args) == "commit":
		return "commit"
	case c.program == s.git:
		return "git"
	case c.program == s.binary && len(c.args) > 0:
		return m2CandidateKind(c.args)
	case filepath.Base(c.program) == executable("go") && slices.Equal(c.args, []string{"interrupt"}):
		return "controller"
	}
	tool, found := runner.LookupTool(strings.TrimSuffix(filepath.Base(c.program), ".exe"))
	if found && slices.Equal(c.args, tool.Probe) {
		return "probe"
	}
	return ""
}

// m2CandidateKind names a command of the candidate by its words.
func m2CandidateKind(args []string) string {
	switch {
	case args[0] == commandInit && slices.Contains(args, "--update"):
		return "update"
	case args[0] == commandInit:
		return "init"
	case args[0] == "setup" && slices.Contains(args, "--uninstall"):
		return "uninstall"
	}
	return args[0]
}

// m2Subcommand is the subcommand of a git command line, after the -c and
// -C options git takes before it.
func m2Subcommand(args []string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == "-c" || args[i] == "-C" {
			i++
			continue
		}
		return args[i]
	}
	return ""
}

// m2Target reports whether a command is the operation the family of a
// lifecycle row judges; a commit counts only through the hooks.
func m2Target(ctx context.Context, t *testing.T, s *suite, step m2Step, c command) bool {
	t.Helper()
	targets := map[string]string{
		"init-unarmed": "init", "init-idempotent": "init", "dispatch-unarmed": "commit",
		"setup-clean": "setup", "setup-idempotent": "setup", "setup-interrupted": "controller",
		"setup-uninstall": "uninstall", "update-preserves-edit": "update", "upgrade-commit": "commit",
	}
	want, found := targets[step.family]
	if !found {
		want = "hook"
	}
	return step.kind == want && (want != "commit" || m2Armed(ctx, t, s, c))
}

// m2CorruptInit applies the corruption of the init rows: a hook the init
// of init-unarmed arms, or the generated asset the second init of
// init-idempotent changes.
func m2CorruptInit(ctx context.Context, t *testing.T, s *suite, step m2Step, c command) {
	t.Helper()
	switch {
	case !step.corrupt:
	case step.family == "init-unarmed":
		m2Write(t, filepath.Join(m2HooksDir(ctx, t, s, c), "pre-commit"), m2HookBody("pre-commit"))
	case step.family == "init-idempotent" && step.count["init"] > 1:
		path := filepath.Join(c.dir, ".editorconfig")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		m2Write(t, path, string(data)+"# changed by a second init\n")
	}
}

// m2Rung answers a direct rung call by the row's name: an older or newer
// pin refuses with both versions and the repair, an equal pin and a
// switched off rung pass silently; corrupt swaps refusal and pass and
// gives a switched off rung a finding.
func m2Rung(name string, corrupt bool) result {
	cut := strings.LastIndex(name, "-")
	rung, suffix := name[:cut], name[cut+1:]
	pins := map[string]string{"older": "0.1.0", "newer": "0.3.0", "equal": "0.1.0"}
	refusal := result{code: 2, stderr: "golaw " + rung + ": golaw.yml pins kit version " + pins[suffix] +
		" and this binary is " + m2Version + ": install the pinned version or run init --update\n"}
	switch {
	case suffix == "disabled" && corrupt:
		return result{code: 1, stdout: "golaw " + rung + ": note.txt:2:1 fail text.dashes em dash (U+2014) in text\n"}
	case suffix == "disabled":
		return result{}
	case (suffix == "equal") == corrupt:
		return refusal
	}
	return result{}
}

// m2Controller stands in for the interrupt controller: the missing tools
// installed, the three markers written and no receipt; corrupt arms the
// repository too, the premature receipt.
func m2Controller(ctx context.Context, t *testing.T, s *suite, c command, corrupt bool) result {
	t.Helper()
	marker := environmentValue(c.env, "GOLAW_M2_MARKER")
	for _, path := range []string{marker, marker + ".release", marker + ".done"} {
		m2Write(t, path, "simulated\n")
	}
	m2InstallTools(t, s, c)
	if corrupt {
		m2Setup(ctx, t, s, c, false)
	}
	return result{stdout: "M2_INSTALL_INTERRUPTED\n"}
}

// m2Through answers the commit a row makes through the hooks: the real
// commit with its hooks routed to an empty directory, since no simulated
// tool can run them, or, when the mode corrupts, the refusal of a rung.
func m2Through(ctx context.Context, t *testing.T, s *suite, c command, step m2Step) result {
	t.Helper()
	switch {
	case step.corrupt && step.family == "upgrade-commit":
		return result{code: 1, stderr: "golaw pre-commit: golaw.yml pins kit version 0.1.0 and this binary is " +
			m2Version + ": install the pinned version or run init --update\n"}
	case step.corrupt && step.family == "dispatch-unarmed":
		return result{code: 1, stdout: "note.txt:2:1 fail text.dashes em dash (U+2014) in text\n"}
	}
	quiet := filepath.Join(s.base, "simulated-hooks")
	if err := os.MkdirAll(quiet, dirMode); err != nil {
		t.Fatal(err)
	}
	c.args = append([]string{"-c", "core.hooksPath=" + quiet}, c.args...)
	return runCommand(ctx, c)
}

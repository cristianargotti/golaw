package commits_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// message is a commit message as two -m parts give it: a subject and a
// body, the body empty for a message of one line.
type message struct {
	subject, body string
}

// command is the command line that makes the commit of the message, each
// part single quoted for the POSIX reading of the shell family.
func (m message) command() string {
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
	command := "git commit -m " + quote(m.subject)
	if m.body != "" {
		command += " -m " + quote(m.body)
	}
	return command
}

// commit is the commit git makes of the same command line: the parts
// joined by a blank line and cleaned with the whitespace mode, because git
// edits no message given with -m.
func (m message) commit() model.Commit {
	text := m.subject + "\n"
	if m.body != "" {
		text += "\n" + m.body + "\n"
	}
	return model.Commit{Message: text, Cleanup: model.CleanupWhitespace}
}

// shellFinds runs a check of the shell family over the command line of the
// message and reports whether it yields the code.
func shellFinds(t *testing.T, c model.Check, code string, m message) bool {
	t.Helper()
	return shellCommandFinds(t, c, code, m.command())
}

// shellCommandFinds runs a check of the shell family over a command line,
// on a tree that holds no file, and reports whether it yields the code.
func shellCommandFinds(t *testing.T, c model.Check, code, command string) bool {
	t.Helper()
	event := &model.Event{Shape: model.ShapeShell, Agent: "tester", Tool: "Bash", Shell: &model.ShellCommand{Command: command, Cwd: "/work", Tool: "Bash"}}
	res, err := c.Run(t.Context(), model.Input{Mode: model.ModeHook, Policy: proof.Policy(), Snapshot: proof.NewTree(nil), Event: event})
	if err != nil || res.Status.Kind != model.StatusMeasured {
		t.Fatalf("%q: %+v, %v", command, res, err)
	}
	return hasCode(res, code)
}

// commitFinds runs a check of the commits family over the commit of the
// message and reports whether it yields the code.
func commitFinds(t *testing.T, c model.Check, code string, m message) bool {
	t.Helper()
	return hasCode(judgeCommit(t, c, m.commit()), code)
}

func hasCode(res model.Result, code string) bool {
	for k := range res.Findings {
		f := &res.Findings[k]
		if f.Code == code {
			return true
		}
	}
	return false
}

// agree holds a law the two families share to one verdict per message:
// the shell check over the command line and the commit check over the
// message git records from it both find the code, or neither does.
func agree(t *testing.T, shellCheck, commitCheck model.Check, code string, rows map[message]bool) {
	t.Helper()
	for m, want := range rows {
		if got := shellFinds(t, shellCheck, code, m); got != want {
			t.Errorf("%s over %q: %v, want %v", shellCheck.Spec().ID, m.command(), got, want)
		}
		if got := commitFinds(t, commitCheck, code, m); got != want {
			t.Errorf("%s over %q: %v, want %v", commitCheck.Spec().ID, m.commit().Message, got, want)
		}
	}
}

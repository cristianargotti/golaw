package coexist_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/coexist"
)

// gateCommand runs the gate the way its installer registers it, and
// foreignCommand is the hook of another tool.
const (
	gateCommand    = "python3 /opt/gate/scripts/standards_gate.py"
	foreignCommand = "python3 /opt/hooks/classify.py"
)

// omitted stands for a group that carries no matcher key at all.
const omitted = "<omitted>"

// toolEvents registers one command before and after the tools a matcher
// names.
func toolEvents(matcher, command string) string {
	group := `{"matcher": "` + matcher + `", "hooks": [{"type": "command", "command": "` + command + `"}]}`
	if matcher == omitted {
		group = `{"hooks": [{"type": "command", "command": "` + command + `"}]}`
	}
	return `{"hooks": {"PreToolUse": [` + group + `], "PostToolUse": [` + group + `]}}`
}

func TestAMatcherOfEveryToolRegistersEveryEvent(t *testing.T) {
	t.Parallel()
	every := []string{"write", "read", "shell", "post-write", "post-shell"}
	cases := map[string]struct {
		settings string
		events   []string
	}{
		"star":            {toolEvents("*", gateCommand), every},
		"empty":           {toolEvents("", gateCommand), every},
		"omitted":         {toolEvents(omitted, gateCommand), every},
		"comma list":      {toolEvents("Read, NotebookEdit", gateCommand), []string{"write", "read", "post-write"}},
		"other shell":     {toolEvents("PowerShell|Glob", gateCommand), []string{"shell", "post-shell"}},
		"foreign on star": {toolEvents("*", foreignCommand), nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			d, err := detect(home(t, tc.settings, registryFor(repo, "")), repo)
			if err != nil || len(d.Registered) != len(tc.events) {
				t.Fatalf("detection = %+v, %v", d, err)
			}
			for _, event := range tc.events {
				if !d.Registered[event] {
					t.Fatalf("event %s not registered: %+v", event, d.Registered)
				}
			}
			if refusal := coexist.Build(d).Refuse(); (refusal == nil) == (len(tc.events) > 0) {
				t.Fatalf("refuse = %v", refusal)
			}
		})
	}
}

func TestARegularExpressionMatcherOfTheGateIsAnError(t *testing.T) {
	t.Parallel()
	for _, matcher := range []string{"Notebook.*", "^Write$", "Write|Edit.*"} {
		_, err := detect(home(t, toolEvents(matcher, gateCommand), ""), t.TempDir())
		if !errors.Is(err, coexist.ErrDetect) || !strings.Contains(err.Error(), `"`+matcher+`"`) {
			t.Fatalf("%s: err = %v", matcher, err)
		}
	}
	d, err := detect(home(t, toolEvents("Notebook.*", foreignCommand), ""), t.TempDir())
	if err != nil || len(d.Registered) != 0 {
		t.Fatalf("a foreign regular expression = %+v, %v", d, err)
	}
}

func TestTheGateIsTheScriptACommandRuns(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		hook string
		gate bool
	}{
		"interpreter":  {`{"command": "python3 /opt/gate/scripts/standards_gate.py"}`, true},
		"exec form":    {`{"command": "python3", "args": ["/opt/gate/scripts/standards_gate.py"]}`, true},
		"wrapped":      {`{"command": "bash -c 'python3 \"/opt/my gate/standards_gate.py\"; true'"}`, true},
		"similar name": {`{"command": "python3 /opt/gate/scripts/not_standards_gate.py"}`, false},
		"echo":         {`{"command": "echo standards_gate.py"}`, false},
		"echoed text":  {`{"command": "echo 'the standards_gate.py gate is retired'"}`, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			settings := `{"hooks": {"PreToolUse": [{"matcher": "Write", "hooks": [` + tc.hook + `]}]}}`
			d, err := detect(home(t, settings, ""), t.TempDir())
			if err != nil || d.Registered["write"] != tc.gate {
				t.Fatalf("detection = %+v, %v", d, err)
			}
		})
	}
}

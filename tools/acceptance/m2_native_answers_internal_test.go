package main

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// m2NativeScript answers a native row: setup and the tools' probes as a
// lifecycle row answers them, the Windows PATH probe, and each installed
// hook invocation, the target, with the answer the row's name and the mode
// set.
func m2NativeScript(t *testing.T, s *suite, name, mode string) *m2Script {
	t.Helper()
	return newM2Script(t, s, m2NativeAnswer(t, s, name, mode))
}

// m2NativeAnswer is the answer of one native row.
func m2NativeAnswer(t *testing.T, s *suite, name, mode string) m2Answer {
	t.Helper()
	return func(ctx context.Context, c command) (m2Reply, bool) {
		switch {
		case m2InstalledHook(s, c):
			return m2Reply{got: m2Installed(t, name, mode, c), target: true}, true
		case len(c.args) > 1 && c.args[0] == "-c" && strings.Contains(c.args[1], "recorder displaced"):
			return m2Reply{got: m2PathProbe(mode)}, true
		}
		switch m2Kind(s, c) {
		case "setup":
			return m2Reply{got: m2Setup(ctx, t, s, c, false)}, true
		case "probe":
			return m2Reply{got: m2ProbeResult(c, false)}, true
		}
		return m2Reply{}, false
	}
}

// m2InstalledHook reports whether a command runs an installed hook file
// through a shell: its first word is a hook lefthook wrote.
func m2InstalledHook(s *suite, c command) bool {
	if c.program == s.binary || c.program == s.git || len(c.args) == 0 {
		return false
	}
	return slices.Contains([]string{"pre-commit", "commit-msg", "pre-push"}, filepath.Base(c.args[0]))
}

// m2Installed answers an installed hook of a native row: the row's red
// evidence or the clean answer, the two swapped when the mode corrupts;
// recorder-started also leaves the marker of a recorder a rung started.
func m2Installed(t *testing.T, name, mode string, c command) result {
	t.Helper()
	switch mode {
	case "broken":
		return result{err: ErrAcceptance}
	case "recorder-started":
		recorders := environmentValue(c.env, "GOLAW_M2_RECORDERS")
		m2Write(t, filepath.Join(recorders, "make.exe.started"), "make.exe\n")
		return result{}
	case "corrupt":
		return m2CorruptInstalled(name)
	}
	if !strings.HasSuffix(name, "-red") {
		return result{}
	}
	return m2NativeEvidence(name)
}

// m2CorruptInstalled is the corrupt answer of an installed hook: a red row
// passes, the push near miss passes unmeasured and every other near miss
// refuses.
func m2CorruptInstalled(name string) result {
	switch {
	case strings.HasSuffix(name, "-red"):
		return result{}
	case name == "installed-pre-push-near":
		return result{stdout: "golaw pre-push: main: govulncheck could not judge it, the pipeline judges it: exit status 2\n"}
	}
	return m2NativeEvidence(name)
}

// m2NativeEvidence is the refusal an installed rung prints for the family
// of a row: the dash finding, the message finding, the advisory, the
// failing staged test or a recorder a rung started.
func m2NativeEvidence(name string) result {
	evidence := map[string]string{
		"installed-pre-commit": "note.txt:2:1 fail text.dashes em dash (U+2014) in text; use a comma, a colon or a hyphen [rules/4.1@1]\n",
		"installed-commit-msg": "golaw commit-msg: commit.conventional: the subject is not a conventional commit\n",
		"installed-pre-push":   "golaw pre-push: main: GO-2026-0001 is reachable through fmt.Println\n",
		"spawn-git":            "--- FAIL: TestStagedFailure\n    staged failure reached\n",
		"windows-shell":        "forbidden rung executable started: make.exe\n",
	}
	return result{code: 1, stderr: evidence[name[:strings.LastIndex(name, "-")]]}
}

// m2PathProbe answers the Windows probe every installed hook runs first:
// silent, or the displacement reason when the mode displaces a recorder.
func m2PathProbe(mode string) result {
	switch mode {
	case "broken":
		return result{err: ErrAcceptance}
	case "displaced-recorder":
		return result{code: 92, stderr: "recorder displaced: make: /usr/bin/make\n"}
	}
	return result{}
}

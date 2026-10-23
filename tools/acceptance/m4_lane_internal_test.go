package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/hook/codex"
)

// m4LaneCredential is the value the tests hand the lane.
const m4LaneCredential = "test-credential"

// m4LaneFake is the machine of the lane and the defect a red case gives it.
type m4LaneFake struct {
	writeDenied, untrusted, execFails, declines bool
}

// codex answers the Codex CLI: the pinned version, and codex exec writing
// what the prompt asks unless the hook blocks the denied write.
func (f m4LaneFake) codex(t *testing.T, c command) result {
	t.Helper()
	if c.args[0] == "--version" {
		return result{stdout: "codex-cli " + codex.PatchGrammar + "\n"}
	}
	if environmentValue(c.env, m4LaneVariable) != m4LaneCredential || strings.Contains(strings.Join(c.args, " "), m4LaneCredential) {
		t.Fatalf("the credential must reach the session through its environment alone: %q", c.args)
	}
	if f.execFails {
		return result{code: 1, stderr: "authentication failed\n"}
	}
	prompt, repo := c.args[len(c.args)-1], c.args[2]
	if strings.Contains(prompt, m4ControlTarget) {
		checkedWrite(t, filepath.Join(repo, filepath.FromSlash(m4ControlTarget)), m4ControlLine+"\n")
	}
	if strings.Contains(prompt, m4DeniedTarget) && f.writeDenied {
		checkedWrite(t, filepath.Join(repo, filepath.FromSlash(m4DeniedTarget)), textDash)
	}
	if strings.Contains(prompt, m4DeniedTarget) && !f.writeDenied && !f.declines {
		return result{stderr: "Command " + m4HookBlock + ": " + m4DeniedTarget + " carries an em dash. Command: *** Add File: " + m4DeniedTarget + "\n"}
	}
	return result{}
}

// doctor answers doctor --json with the trust and the dispatch lines of the
// Codex report.
func (f m4LaneFake) doctor(t *testing.T) result {
	t.Helper()
	trusted := doctor.Line{ID: lineTrusted, Key: "codex/user", State: "ok", Code: "trusted"}
	if f.untrusted {
		trusted.State, trusted.Code = "red", "untrusted"
	}
	dispatch := doctor.Line{ID: lineDispatch, Key: "codex/user", State: "ok", Code: "observed"}
	report, err := json.Marshal(doctor.Report{Schema: 1, Lines: []doctor.Line{trusted, dispatch}})
	if err != nil {
		t.Fatal(err)
	}
	empty := `{"schema":1,"lines":[]}`
	return result{stdout: empty + "\n" + string(report) + "\n" + empty + "\n"}
}

// m4LaneSuite is a suite over a module root of its own whose commands the
// fake answers.
func m4LaneSuite(t *testing.T, f m4LaneFake) *suite {
	t.Helper()
	root := testRoot(t)
	s := newSuite(t.TempDir(), t.TempDir(), simulatedExecutor(t, root), func(name string) (string, error) { return name, nil })
	base := s.execute
	s.execute = func(ctx context.Context, c command) result {
		switch {
		case c.program == agentCodex:
			return f.codex(t, c)
		case len(c.args) == 4 && c.args[0] == commandAgent:
			m4Installed(t, environmentValue(c.env, "HOME"), c.program, s.m4System())
			return result{}
		case len(c.args) == 2 && c.args[0] == commandDoctor:
			return f.doctor(t)
		}
		return base(ctx, c)
	}
	return s
}

// m4Facts writes a facts file and returns its path.
func m4Facts(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "facts.json")
	checkedWrite(t, path, body)
	return path
}

// m4CompleteFacts is a facts file with every fact.
func m4CompleteFacts(t *testing.T) string {
	t.Helper()
	return m4Facts(t, `{"date":"2026-10-21","owner":"the kit owner","environment":"a macOS workstation","credential_boundary":"an API key with a limit",`+
		`"spending_limit":"1 USD","schedule":"before each tag","retention":"kept in the repository"}`)
}

func TestM4LaneWritesTheRecordOfBothCases(t *testing.T) {
	t.Parallel()
	s := m4LaneSuite(t, m4LaneFake{})
	item := s.codexLane(t.Context(), m4CompleteFacts(t), m4LaneCredential, liveOS+"/"+liveArch)
	if item.status != statusPass || !strings.Contains(item.detail, m4LiveRecordPath) {
		t.Fatalf("the lane = %+v", item)
	}
	record, err := decodeLiveRecord(filepath.Join(s.root, filepath.FromSlash(m4LiveRecordPath)))
	if err != nil {
		t.Fatal(err)
	}
	if err := validCodexRecord(record); err != nil || record.Binary != simulatedDigest() || record.Owner != "the kit owner" || record.Date != "2026-10-21" {
		t.Fatalf("record %+v: %v", record, err)
	}
}

func TestM4LaneWritesNothingWhenACaseFails(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, want string
		fake       m4LaneFake
	}{
		{"red the denied file written", "not the denied write", m4LaneFake{writeDenied: true}},
		{"red a hook Codex would not run", "step 3 of the lane preparation", m4LaneFake{untrusted: true}},
		{"red a session that fails", "codex exec exited 1", m4LaneFake{execFails: true}},
		{"red a model that declines while the hook ran", "no hook denial of " + m4DeniedTarget, m4LaneFake{declines: true}},
	} {
		s := m4LaneSuite(t, tc.fake)
		item := s.codexLane(t.Context(), m4CompleteFacts(t), m4LaneCredential, liveOS+"/"+liveArch)
		if item.status != statusFail || !strings.Contains(item.detail, tc.want) {
			t.Errorf("%s: %+v", tc.name, item)
		}
		if _, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(m4LiveRecordPath))); !os.IsNotExist(err) {
			t.Errorf("%s: a record was written: %v", tc.name, err)
		}
	}
}

func TestM4LaneSkipsWhereItCannotRunUnattended(t *testing.T) {
	t.Parallel()
	s := m4LaneSuite(t, m4LaneFake{})
	for _, tc := range []struct {
		name, credential, platform, facts, status, want string
	}{
		{"skip off macOS", m4LaneCredential, "linux/amd64", "", statusSkip, "darwin/arm64"},
		{"skip without a credential", "", liveOS + "/" + liveArch, "", statusSkip, "cannot run unattended"},
		{"red a facts file missing a fact", m4LaneCredential, liveOS + "/" + liveArch, `{"owner":"x"}`, statusFail, "misses a fact"},
		{"red a facts file with another key", m4LaneCredential, liveOS + "/" + liveArch, `{"budget":"x"}`, statusFail, "unknown field"},
	} {
		item := s.codexLane(t.Context(), m4Facts(t, tc.facts+" "), tc.credential, tc.platform)
		if item.status != tc.status || item.required != (tc.status == statusFail) || !strings.Contains(item.detail, tc.want) {
			t.Errorf("%s: %+v", tc.name, item)
		}
	}
	var out bytes.Buffer
	live, only := m4Facts(t, "{}"), false
	if code, routed := (m4Flags{only: &only, live: &live}).route(t.Context(), &out, s, false); !routed || code > 1 ||
		!strings.Contains(out.String(), "\t"+m4LaneRow+"\t") {
		t.Fatalf("the lane flag = %d %v\n%s", code, routed, out.String())
	}
}

func TestM4LaneProcedureNamesWhatTheLaneDoes(t *testing.T) {
	t.Parallel()
	body, err := readFile(filepath.Join(testRoot(t), filepath.FromSlash(m4LiveProcedure)))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"go run ./tools/acceptance -" + m4LaneFlag, m4LaneVariable, m4LiveRecordPath, m4DeniedTarget, m4ControlTarget,
		m4ControlLine, "codex-cli " + codex.PatchGrammar, "`date`", "owner", "environment", "credential_boundary", "spending_limit",
		"schedule", "retention", caseCodexDenied, caseCodexControl, "SKIP", "agent -p", m4HookBlock,
	} {
		if !strings.Contains(string(body), fragment) {
			t.Errorf("%s lacks %q", m4LiveProcedure, fragment)
		}
	}
}

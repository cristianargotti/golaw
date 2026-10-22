package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// codexDenyOf is the pre denial of the contract for a reason.
func codexDenyOf(reason string) result {
	got := codexDeny()
	got.stderr = reason + "\n"
	got.stdout = strings.Replace(got.stdout, sampleReason, reason, 1)
	return got
}

// m4FaultAnswer answers a shell of the failure rows as the fault and the
// real binary answer it: the timeout fault never answers within the host
// deadline, the crash exits 2 with its trace, a stalled git makes the
// internal deadline deny, and golaw denies every input it cannot judge.
func m4FaultAnswer(s *suite, c command) result {
	cursor := strings.HasSuffix(c.args[len(c.args)-1], "hook cursor")
	mode, err := os.ReadFile(s.binary + ".mode")
	if err != nil {
		mode = nil
	}
	slow := filepath.Base(filepath.SplitList(environmentValue(c.env, envPath))[0]) == m4SlowGitDir
	switch {
	case string(mode) == faultTimeout:
		return result{err: context.DeadlineExceeded}
	case string(mode) == faultCrash:
		return result{code: 2, stderr: "panic: controlled crash\n\ngoroutine 1 [running]:\n"}
	case cursor && slow:
		return result{stdout: `{"permission":"deny","user_message":"` + sampleHuman + `","agent_message":"cannot judge: internal deadline passed"}` + "\n"}
	case slow:
		return codexDenyOf("cannot judge: internal deadline passed")
	case cursor:
		return result{stdout: `{"permission":"deny","user_message":"` + sampleHuman + `","agent_message":"` + sampleReason + `"}` + "\n"}
	}
	return codexDeny()
}

// m4FaultSuite is a Linux replay whose shells answer through m4FaultAnswer
// after mutate.
func m4FaultSuite(t *testing.T, mutate func(command, result) result) *suite {
	t.Helper()
	s := m4ReplaySuite(t)
	s.execute = m4Script(s, func(c command) result {
		got := m4FaultAnswer(s, c)
		if mutate != nil {
			got = mutate(c, got)
		}
		return got
	})
	return s
}

func TestM4FailureRowsRecordEveryHostFailure(t *testing.T) {
	t.Parallel()
	s := m4FaultSuite(t, nil)
	rows := s.m4FailureRows(t.Context(), false)
	if got := strings.Join(m4RowNames(rows), ","); got != "codex-host-timeout PASS,cursor-host-timeout PASS,codex-empty-stderr PASS,slow-path PASS" {
		t.Fatalf("rows = %+v", rows)
	}
	if !strings.Contains(rows[0].detail, "fail open") || !strings.Contains(rows[1].detail, "failClosed makes Cursor block") {
		t.Fatalf("details = %q, %q", rows[0].detail, rows[1].detail)
	}
	body, err := readFile(s.binary)
	if err != nil || string(body) != "simulated executable" {
		t.Fatalf("the binary was not put back: %q, %v", body, err)
	}
	if _, err := os.Stat(s.binary + ".mode"); !os.IsNotExist(err) {
		t.Fatalf("a fault mode was left behind: %v", err)
	}
}

// m4FailureMutations are the answers a host would not survive, each with
// the row it fails and the part of the detail it yields.
func m4FailureMutations() []struct {
	name, row, want string
	mutate          func(command, result) result
} {
	return []struct {
		name, row, want string
		mutate          func(command, result) result
	}{
		{"red a hook answering within the timeout", "codex-host-timeout", "within the host timeout", func(_ command, got result) result {
			if got.err != nil {
				return result{}
			}
			return got
		}},
		{"red a crash with an empty stderr", m4EmptyStderrRow, "a crash", func(_ command, got result) result {
			if strings.HasPrefix(got.stderr, "panic") {
				got.stderr = ""
			}
			return got
		}},
		{"red a malformed input that fails open", m4EmptyStderrRow, "one reason line", func(c command, got result) result {
			if c.input == "not-json" {
				got.stderr = ""
			}
			return got
		}},
		{"red a slow path denial without the deadline", m4SlowPathRow, "names no deadline", func(_ command, got result) result {
			if strings.Contains(got.stderr, "deadline") {
				return codexDeny()
			}
			return got
		}},
	}
}

func TestM4FailureRowsFailWhatAHostWouldNotSurvive(t *testing.T) {
	t.Parallel()
	for _, tc := range m4FailureMutations() {
		s := m4FaultSuite(t, tc.mutate)
		rows := s.m4FailureRows(t.Context(), false)
		index := slices.IndexFunc(rows, func(item row) bool { return item.name == tc.row })
		if index < 0 || rows[index].status != statusFail || !strings.Contains(rows[index].detail, tc.want) {
			t.Errorf("%s: %+v", tc.name, rows)
		}
	}
	s := m4FaultSuite(t, nil)
	s.execute = func(context.Context, command) result { return result{code: 1} }
	for _, item := range s.m4FailureRows(t.Context(), false) {
		if item.status != statusFail {
			t.Errorf("a fault that does not build: %+v", item)
		}
	}
}

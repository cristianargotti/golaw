package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// The m4-failure category (section 10.3 item 4) and the directory of the
// git that stalls.
const (
	m4FailureCategory = "m4-failure"
	m4SlowGitDir      = "m4-slowgit"
	m4SlowMode        = "slowgit"
	m4EmptyStderrRow  = "codex-empty-stderr"
	m4SlowPathRow     = "slow-path"
	m4HostTimeoutRow  = "-host-timeout"
)

// m4FailureRows is the m4-failure category: the host timeout of Codex and
// of Cursor, each recorded as its host answers it, every exit 2 of the
// Codex command carrying a reason line, and the internal deadline denying
// a pre event on both agents.
func (s *suite) m4FailureRows(ctx context.Context, _ bool) []row {
	fault, err := s.buildFault(ctx, filepath.Join(s.root, dirTools, dirAcceptance, m2Testdata, "fault.go.txt"))
	var repo string
	if err == nil {
		repo, err = s.repository(ctx, "m4-failure", true)
	}
	if err != nil {
		names := []string{agentCodex + m4HostTimeoutRow, agentCursor + m4HostTimeoutRow, m4EmptyStderrRow, m4SlowPathRow}
		rows := make([]row, 0, len(names))
		for _, name := range names {
			rows = append(rows, measuredM4(m4FailureCategory, name, err))
		}
		return rows
	}
	return []row{
		s.m4HostTimeout(ctx, agentCodex, repo, fault),
		s.m4HostTimeout(ctx, agentCursor, repo, fault),
		measuredM4(m4FailureCategory, m4EmptyStderrRow, s.m4EmptyStderr(ctx, repo, fault)),
		measuredM4(m4FailureCategory, m4SlowPathRow, s.m4SlowPath(ctx, repo, fault)),
	}
}

// m4HostTimeout replaces the binary with a fault that sleeps past the
// installed timeout and runs the installed command under that timeout, as
// the host does. The row observes the fault; what the host does then is
// its documented answer, recorded and never inferred from a live session:
// Codex has no fail closed setting and runs the tool (D6), Cursor's
// failClosed blocks it.
func (s *suite) m4HostTimeout(ctx context.Context, agent, repo, fault string) row {
	name := agent + m4HostTimeoutRow
	t := s.m4Shared(repo)
	payload, err := s.m4Sample(agent, m4ShellSample(agent), repo, nil)
	if err != nil {
		return measuredM4(m4FailureCategory, name, err)
	}
	event, tool := m4RouteOf(agent, payload)
	entry, err := s.m4Golaw(t, agent, event, tool)
	if err != nil {
		return measuredM4(m4FailureCategory, name, err)
	}
	err = s.m4WithFault(faultTimeout, fault, func() error {
		child, cancel := context.WithTimeout(ctx, time.Duration(entry.Handler.Timeout)*time.Second)
		defer cancel()
		replies, runErr := s.m4Through(child, t, agent, m4Command(agent, s.m4System(), entry.Handler), payload)
		for _, reply := range replies {
			if runErr == nil && reply.got.err == nil {
				runErr = fmt.Errorf("the hook answered through %s within the host timeout: %w", reply.shell, ErrAcceptance)
			}
		}
		return runErr
	})
	item := measuredM4(m4FailureCategory, name, err)
	if err == nil {
		item.detail = m4TimeoutDetail(agent, entry.Handler.Timeout)
	}
	return item
}

// m4TimeoutDetail records what the host does once the fault is observed.
func m4TimeoutDetail(agent string, seconds int) string {
	if agent == agentCodex {
		return fmt.Sprintf("fault observed past the %d second timeout; Codex has no fail closed setting, so the host runs the tool: fail open, recorded (D6), never a denial", seconds)
	}
	return fmt.Sprintf("fault observed past the %d second timeout; the installed failClosed makes Cursor block the action, documented and not observed live", seconds)
}

// m4WithFault runs fn with the binary replaced by the fault in a mode and
// puts the binary back whatever fn answers.
func (s *suite) m4WithFault(mode, fault string, fn func() error) error {
	original, err := readFile(s.binary)
	if err != nil {
		return err
	}
	replacement, err := readFile(fault)
	if err != nil {
		return err
	}
	err = s.replaceWithFault(mode, replacement)
	if err == nil {
		err = fn()
	}
	restoreErr := writeFile(s.binary, original, execMode)
	removeErr := os.Remove(s.binary + ".mode")
	if errors.Is(removeErr, fs.ErrNotExist) {
		removeErr = nil
	}
	return errors.Join(err, restoreErr, removeErr)
}

// m4CannotJudge are the inputs the Codex command cannot judge, each named.
func (s *suite) m4CannotJudge(repo string) (map[string][]byte, error) {
	foreign, err := s.m4Sample(agentCursor, m4ShellSample(agentCursor), repo, nil)
	if err != nil {
		return nil, err
	}
	return map[string][]byte{
		"malformed": []byte("not-json"), "empty": nil, "truncated": []byte(`{"tool_name":"apply_patch"`),
		"oversized":        append(bytes.Repeat([]byte(" "), payloadLimit+1), "{}"...),
		"foreign-envelope": foreign, "second-object": []byte("{}{}"),
	}, nil
}

// m4EmptyStderr sends the Codex command every input golaw cannot judge,
// each answered by the pre denial of the contract, and a crash, which
// never exits 0 and always carries its reason on stderr: Codex fails open
// on an exit 2 whose stderr is empty (survey section 3.1), and under
// powershell -Command a crash's exit 2 reads as 1, a failed hook.
func (s *suite) m4EmptyStderr(ctx context.Context, repo, fault string) error {
	inputs, err := s.m4CannotJudge(repo)
	if err != nil {
		return err
	}
	t := s.m4Shared(repo)
	for _, name := range slices.Sorted(maps.Keys(inputs)) {
		replies, invokeErr := s.m4Invoke(ctx, t, agentCodex, eventPre, toolBash, inputs[name])
		if invokeErr == nil {
			invokeErr = m4Judged(agentCodex, eventPre, m4Deny, replies)
		}
		if invokeErr != nil {
			return fmt.Errorf("%s: %w", name, invokeErr)
		}
	}
	return s.m4WithFault(faultCrash, fault, func() error {
		replies, invokeErr := s.m4Invoke(ctx, t, agentCodex, eventPre, toolBash, []byte("{}"))
		for _, reply := range replies {
			got := reply.got
			if invokeErr == nil && (got.err != nil || got.code == 0 || strings.TrimSpace(got.stderr) == "") {
				invokeErr = fmt.Errorf("a crash through %s exited %d with stderr %q: %w", reply.shell, got.code, got.stderr, ErrAcceptance)
			}
		}
		return invokeErr
	})
}

// m4SlowGit writes the git that stalls on ls-files and runs the real git
// for every other command, the fault the M1 slow path uses.
func (s *suite) m4SlowGit(fault string) (string, error) {
	dir := filepath.Join(s.base, m4SlowGitDir)
	git := filepath.Join(dir, executable(m2GitCommand))
	body, err := readFile(fault)
	if err != nil {
		return "", err
	}
	files := map[string][]byte{git: body, git + ".mode": []byte(m4SlowMode), git + ".real": []byte(s.git)}
	for path, content := range files {
		if err := writeFile(path, content, execMode); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// m4SlowPath puts the stalling git first on PATH: the hook cannot reach
// its answer before the host would kill it, so the internal deadline
// denies the pre event on both agents, each denial naming the deadline.
func (s *suite) m4SlowPath(ctx context.Context, repo, fault string) error {
	dir, err := s.m4SlowGit(fault)
	if err != nil {
		return err
	}
	t := s.m4Shared(repo)
	t.env = prependPath(s.env, dir)
	for _, agent := range []string{agentCodex, agentCursor} {
		payload, sampleErr := s.m4Sample(agent, m4ShellSample(agent), repo, nil)
		if sampleErr != nil {
			return sampleErr
		}
		child, cancel := context.WithTimeout(ctx, slowTimeout)
		answer, reasonErr := s.m4Reason(child, t, agent, m4Deny, payload)
		cancel()
		if reasonErr != nil {
			return reasonErr
		}
		if !strings.Contains(answer.Reason, "deadline") {
			return fmt.Errorf("the %s denial %q names no deadline: %w", agent, answer.Reason, ErrAcceptance)
		}
	}
	return nil
}

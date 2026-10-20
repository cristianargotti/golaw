package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	processTimeout       = 2 * time.Minute
	m2ProcessTimeout     = 20 * time.Minute
	m2MeasurementTimeout = 50 * time.Minute
	wantAllow            = "allow"
	wantDeny             = "deny"
	wantError            = "error"
	wantPost             = "post"
	wantFailure          = "failure"
	wantPostAllow        = "post-allow"
	eventPre             = "PreToolUse"
	eventPost            = "PostToolUse"
	eventFailure         = "PostToolUseFailure"
	agentClaude          = "claude"
	toolWrite            = "Write"
	toolEdit             = "Edit"
	toolNotebook         = "NotebookEdit"
	toolRead             = "Read"
	toolBash             = "Bash"
	toolPowerShell       = "PowerShell"
)

type command struct {
	program string
	argv0   string
	args    []string
	dir     string
	env     []string
	input   string
	timeout time.Duration
}

type result struct {
	stdout string
	stderr string
	code   int
	err    error
}

type executor func(context.Context, command) result

type locator func(string) (string, error)

func runCommand(ctx context.Context, c command) result {
	return runCommandWithin(ctx, c, processTimeout)
}

func runM2Command(ctx context.Context, c command) result {
	return runCommandWithin(ctx, c, m2ProcessTimeout)
}

func runCommandWithin(ctx context.Context, c command, limit time.Duration) result {
	if c.timeout > 0 {
		limit = c.timeout
	}
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.program, c.args...) //nolint:gosec // Only fixed build, installer, and hook invocations enter this executor.
	if c.argv0 != "" {
		cmd.Args[0] = c.argv0
	}
	cmd.Dir, cmd.Env, cmd.Stdin = c.dir, c.env, strings.NewReader(c.input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	got := result{stdout: stdout.String(), stderr: stderr.String()}
	var exited *exec.ExitError
	switch {
	case ctx.Err() != nil:
		got.err = fmt.Errorf("executable deadline: %w", ctx.Err())
	case errors.As(err, &exited):
		got.code = exited.ExitCode()
	case err != nil:
		got.err = fmt.Errorf("start executable: %w", err)
	}
	return got
}

func success(got result) error {
	if got.err != nil {
		return fmt.Errorf("execute process: %w", got.err)
	}
	if got.code != 0 {
		return fmt.Errorf("exit %d: %s: %w", got.code,
			strings.TrimSpace(got.stdout+"\n"+got.stderr), ErrAcceptance)
	}
	return nil
}

type answer struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
	Human    string `json:"systemMessage"`
	Hook     struct {
		Event      string `json:"hookEventName"`
		Decision   string `json:"permissionDecision"`
		Reason     string `json:"permissionDecisionReason"`
		Additional string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
}

func expect(got result, want string) error {
	if got.err != nil {
		return fmt.Errorf("execute installed invocation: %w", got.err)
	}
	if want == wantError {
		if got.code == 2 && strings.TrimSpace(got.stderr) != "" && got.stdout == "" {
			return nil
		}
		return fmt.Errorf("exit 2 requires stderr and empty stdout: %w", ErrAcceptance)
	}
	if got.code != 0 || got.stderr != "" {
		return fmt.Errorf("unexpected exit %d or stderr %q: %w", got.code, strings.TrimSpace(got.stderr), ErrAcceptance)
	}
	if validAnswer(got, want) {
		return nil
	}
	return fmt.Errorf("answer %q does not satisfy %q: %w", strings.TrimSpace(got.stdout), want, ErrAcceptance)
}

func validAnswer(got result, want string) bool {
	if want == wantAllow || want == wantPostAllow {
		return got.err == nil && got.code == 0 && got.stdout == "" && got.stderr == ""
	}
	var decoded answer
	if err := json.Unmarshal([]byte(got.stdout), &decoded); err != nil {
		return false
	}
	switch want {
	case wantDeny:
		return decoded.Hook.Event == eventPre && decoded.Hook.Decision == wantDeny &&
			decoded.Hook.Reason != "" && decoded.Human != "" && decoded.Human != decoded.Hook.Reason
	case wantPost:
		return decoded.Decision == "block" && decoded.Reason != ""
	case wantFailure:
		return decoded.Hook.Event == eventFailure && decoded.Hook.Additional != ""
	default:
		return false
	}
}

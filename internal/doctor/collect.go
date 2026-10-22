package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/install"
)

// Input separates command configuration from evidence gathered by diagnostics.
// Agent and Scope name the report; ConfigDir is where the host of the agent
// reads its settings, Home the home directory of the person and StateDir
// the state directory receipts, observations and captures share. Language
// is the language of the human channel of the policy in force; Captures
// are the key names a hook process recorded for the repository (A4), and
// CaptureError why they could not be read.
type Input struct {
	Root, ReceiptPath, RunningBinary           string
	PolicyDigest, ProfileDigest, PinnedVersion string
	RequiredRead                               []string
	Environment                                AgentEnvironment
	Dispatch                                   Dispatch
	Agent, Scope, ConfigDir, Home, StateDir    string
	Language                                   string
	Captures                                   []hook.Capture
	CaptureError                               string
}

// VersionRunner permits native process behavior to be replaced in unit tests.
type VersionRunner interface {
	Version(ctx context.Context, environment AgentEnvironment) (string, error)
}

// NativeRunner invokes the resolved executable directly, without a shell.
type NativeRunner struct{}

// Version resolves the agent through the observed search path, because the
// terminal PATH may name another executable.
func (NativeRunner) Version(ctx context.Context, environment AgentEnvironment) (string, error) {
	return runVersion(ctx, agentClaude, environment)
}

// runVersion runs the agent of a command, resolved through the observed
// search path, with --version, and keeps what it printed.
func runVersion(ctx context.Context, agent string, environment AgentEnvironment) (string, error) {
	path, err := ResolveAgentBinary(agent, environment)
	if err != nil {
		return "", err
	}
	//nolint:gosec // The path was resolved through the agent's observed search path.
	command := exec.CommandContext(ctx, path, "--version")
	command.Dir = filepath.Dir(path)
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("read agent version: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// Collect retains independent evidence even when another observation fails.
func Collect(ctx context.Context, input Input, enrollment EnrollmentReader, runner VersionRunner) Snapshot {
	snapshot := collectFiles(input)
	snapshot.RequiredRead = input.RequiredRead
	snapshot.PinnedVersion = input.PinnedVersion
	snapshot.Dispatch = input.Dispatch
	snapshot.Build = RunningMetadata()
	snapshot.Binary = CompareBinary("golaw", input.Environment, input.RunningBinary)
	if runner != nil {
		version, err := runner.Version(ctx, input.Environment)
		snapshot.ClaudeVersion = version
		if err != nil {
			snapshot.ClaudeVersion, snapshot.ClaudeVersionError = "", err.Error()
		}
	}
	_, identity, err := install.ProjectIdentity(input.Root)
	if err != nil {
		snapshot.EnrollmentError = err.Error()
		return snapshot
	}
	if enrollment == nil {
		snapshot.EnrollmentError = "enrollment reader unavailable"
		return snapshot
	}
	snapshot.Enrollment, err = enrollment.ReadEnrollment(
		ctx, identity, input.PolicyDigest, input.ProfileDigest,
	)
	if err != nil {
		snapshot.EnrollmentError = err.Error()
	}
	return snapshot
}

func readText(path string) string {
	data, err := os.ReadFile(path) //nolint:gosec // Paths come from the installation receipt.
	if err != nil {
		return ""
	}
	return string(data)
}

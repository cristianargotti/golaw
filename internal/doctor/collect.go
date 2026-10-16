package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/install"
)

// Input separates command configuration from evidence gathered by diagnostics.
type Input struct {
	Root, ReceiptPath, RunningBinary           string
	PolicyDigest, ProfileDigest, PinnedVersion string
	RequiredRead                               []string
	Environment                                AgentEnvironment
}

// VersionRunner permits native process behavior to be replaced in unit tests.
type VersionRunner interface {
	Version(ctx context.Context, environment AgentEnvironment) (string, error)
}

// NativeRunner invokes the resolved executable directly, without a shell.
type NativeRunner struct{}

// Version obtains the installed agent version using the observed search path.
func (NativeRunner) Version(ctx context.Context, environment AgentEnvironment) (string, error) {
	path, err := ResolveAgentBinary(agentClaude, environment)
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
	return string(output), nil
}

// Collect retains independent evidence even when another observation fails.
func Collect(ctx context.Context, input Input, enrollment EnrollmentReader, runner VersionRunner) Snapshot {
	snapshot := collectFiles(input)
	snapshot.RequiredRead = input.RequiredRead
	snapshot.PinnedVersion = input.PinnedVersion
	snapshot.Build = RunningMetadata()
	snapshot.Binary = CompareBinary("golaw", input.Environment, input.RunningBinary)
	if runner != nil {
		version, err := runner.Version(ctx, input.Environment)
		snapshot.ClaudeVersion = version
		if err != nil {
			snapshot.ClaudeVersion = err.Error()
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

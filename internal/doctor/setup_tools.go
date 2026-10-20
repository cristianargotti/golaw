package doctor

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/setup"
)

const setupVulnTool = "govulncheck"

func collectSetupTools(ctx context.Context, env []string, recorded []setup.ToolRecord) []ToolStatus {
	tools := runner.Toolchain()
	statuses := make([]ToolStatus, 0, len(tools))
	for _, tool := range tools {
		statuses = append(statuses, collectSetupTool(ctx, tool, runner.UserEnv(env), recorded))
	}
	return statuses
}

func collectSetupTool(ctx context.Context, tool runner.Tool, env []string, recorded []setup.ToolRecord) ToolStatus {
	status := ToolStatus{Name: tool.Name}
	path, err := exec.LookPath(tool.Name)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Path = path
	if tool.Name == setupVulnTool {
		// Its version probe contacts the vulnerability database.
		return setupRecordedTool(status, recorded)
	}
	if _, err := runner.Verify(ctx, tool, env); err != nil {
		return collectSetupProbe(ctx, tool, env, status)
	}
	status.Version = tool.Version
	return status
}

// setupRecordedTool takes the version setup recorded for a tool doctor
// never starts. That version describes only the path setup verified, so
// another path PATH resolves now is a binary doctor cannot vouch for.
func setupRecordedTool(status ToolStatus, recorded []setup.ToolRecord) ToolStatus {
	index := slices.IndexFunc(recorded, func(item setup.ToolRecord) bool { return item.Name == status.Name })
	if index < 0 {
		return status
	}
	if filepath.Clean(recorded[index].Path) != filepath.Clean(status.Path) {
		status.Error = fmt.Sprintf("PATH resolves %s at %s, setup recorded it at %s", status.Name, status.Path, recorded[index].Path)
		return status
	}
	status.Version = recorded[index].Version
	return status
}

func collectSetupProbe(ctx context.Context, tool runner.Tool, env []string, status ToolStatus) ToolStatus {
	out, err := runner.Run(ctx, runner.Command{Name: status.Path, Args: tool.Probe, Env: env})
	status.Version = setupProbeLine(string(out.Stdout))
	switch {
	case err != nil:
		status.Error = err.Error()
	case out.Code != 0:
		status.Error = fmt.Sprintf("version probe exited %d: %s", out.Code, setupProbeLine(string(out.Stderr)))
	case status.Version == "":
		status.Error = "version probe returned no version"
	}
	return status
}

func setupProbeLine(output string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(output), "\n")
	return strings.TrimSpace(line)
}

func setupToolLines(facts SetupFacts) []Line {
	if facts.ReceiptState != setupPresent {
		return nil
	}
	lines := make([]Line, 0, len(facts.Tools))
	for _, status := range facts.Tools {
		lines = append(lines, setupToolLine(status))
	}
	return lines
}

func setupToolLine(status ToolStatus) Line {
	line := Line{
		ID: idSetupTools, Key: status.Name, State: stateOK, Code: "setup-tool-available",
		Detail: fmt.Sprintf("%s: %s", status.Path, setupProbeLine(status.Version)),
	}
	tool, known := runner.LookupTool(status.Name)
	switch {
	case status.Path == "":
		line.State, line.Detail = setupMissing, "is not on PATH"
	case status.Error != "" || !known:
		line.State, line.Detail = stateUnknown, "could not be verified"
	case status.Name == setupVulnTool && status.Version != tool.Version:
		line.State, line.Detail = "version", "has recorded version "+setupRecordedVersion(status.Version)+" at "+status.Path
	case !tool.Matches(status.Version):
		line.State, line.Detail = "version", "reports version "+setupProbeLine(status.Version)+" at "+status.Path
	}
	if status.Name == setupVulnTool && status.Path != "" && status.Error == "" {
		line.Detail += " (version recorded by setup; no probe)"
	}
	if line.State != stateOK {
		line.Code = "setup-tool-unavailable"
		line.Detail = setupToolRemedy(status, tool, line.Detail)
	}
	return line
}

func setupRecordedVersion(version string) string {
	if version == "" {
		return "absent"
	}
	return setupProbeLine(version)
}

func setupToolRemedy(status ToolStatus, tool runner.Tool, detail string) string {
	label := strings.TrimSpace(status.Name + " " + tool.Version)
	detail = label + " " + detail
	if status.Error != "" {
		detail += ": " + setupProbeLine(status.Error)
	}
	return detail + "; run golaw setup, then put GOBIN on PATH"
}

// Package doctor reports evidence without treating installation as trust.
package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	stateOK          = "ok"
	stateRed         = "red"
	stateUnknown     = "unknown"
	sourceAgent      = "agent-observed"
	codeHookFailOpen = "hook-fail-open"
	idPython         = "doctor.python"
	idArtifacts      = "doctor.artifacts"
	idHooks          = "doctor.hooks"
	agentClaude      = "claude"
)

// AgentEnvironment avoids mistaking terminal configuration for agent evidence.
type AgentEnvironment struct {
	Source     string `json:"source"`
	Path       string `json:"path"`
	PathExt    string `json:"path_ext"`
	WorkingDir string `json:"working_dir"`
}

// BinaryEvidence records both names before their identity is assessed.
type BinaryEvidence struct {
	Source      string `json:"source"`
	HookPath    string `json:"hook_path"`
	RunningPath string `json:"running_path"`
	Error       string `json:"error"`
}

// ObserveEnvironment retains only values needed to reproduce executable lookup.
func ObserveEnvironment(environment []string, workingDir string) AgentEnvironment {
	out := AgentEnvironment{Source: sourceAgent, WorkingDir: workingDir}
	for _, entry := range environment {
		key, value, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		switch {
		case strings.EqualFold(key, "PATH"):
			out.Path = value
		case strings.EqualFold(key, "PATHEXT"):
			out.PathExt = value
		}
	}
	return out
}

// ResolveAgentBinary supports the bare executable names used by the M1 plugin.
func ResolveAgentBinary(command string, environment AgentEnvironment) (string, error) {
	if environment.Source != sourceAgent {
		return "", fmt.Errorf("resolve %s: %w", command, ErrEnvironment)
	}
	if strings.ContainsAny(command, `/\`) || filepath.Ext(command) != "" {
		return "", fmt.Errorf("bare command required: %w", ErrBinary)
	}
	for _, directory := range filepath.SplitList(environment.Path) {
		if !filepath.IsAbs(directory) {
			directory = filepath.Join(environment.WorkingDir, directory)
		}
		for _, suffix := range executableSuffixes(environment.PathExt) {
			candidate := filepath.Join(directory, command+suffix)
			if !executable(candidate) {
				continue
			}
			canonical, err := filepath.EvalSymlinks(candidate)
			if err != nil {
				return "", fmt.Errorf("resolve executable: %w", err)
			}
			return canonical, nil
		}
	}
	return "", fmt.Errorf("resolve %s: %w", command, ErrBinary)
}

func executableSuffixes(pathExt string) []string {
	if runtime.GOOS != "windows" {
		return []string{""}
	}
	if pathExt == "" {
		pathExt = ".COM;.EXE;.BAT;.CMD"
	}
	return strings.Split(pathExt, ";")
}

func executable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	const executeBits = 0o111
	return runtime.GOOS == "windows" || info.Mode()&executeBits != 0
}

// CompareBinary uses filesystem identity instead of platform-wide case folding.
func CompareBinary(command string, environment AgentEnvironment, running string) BinaryEvidence {
	out := BinaryEvidence{Source: environment.Source}
	resolved, err := ResolveAgentBinary(command, environment)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	out.HookPath = resolved
	canonical, err := filepath.EvalSymlinks(running)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	out.RunningPath = canonical
	left, leftErr := os.Stat(resolved)
	right, rightErr := os.Stat(canonical)
	if leftErr != nil || rightErr != nil {
		out.Error = "binary identity cannot be measured"
		return out
	}
	if os.SameFile(left, right) {
		out.HookPath = canonical
	}
	return out
}

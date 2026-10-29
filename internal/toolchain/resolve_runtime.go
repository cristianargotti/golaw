package toolchain

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/cristianargotti/golaw/internal/runner"
)

// RuntimeStatus is what a runtime prerequisite answered on this PATH.
type RuntimeStatus struct {
	Runtime Runtime
	Path    string // the executable that answered, empty when none did
	Version string // as read from its first line, empty when none was
	OK      bool
	Detail  string // English, names the bounds and what PATH answered
}

// runtimeVersion reads the first version a runtime prints: v24.21.0 reads
// 24.21.0 and Python 3.12.1 reads 3.12.1.
var runtimeVersion = regexp.MustCompile(`\d+\.\d+(?:\.\d+)?`)

// executableBits are the permission bits any of which makes a file
// executable off Windows.
const executableBits = 0o111

// Runtime probes a runtime prerequisite on the resolver's PATH, the first
// executable of its names that is found.
func (r Resolver) Runtime(ctx context.Context, rt Runtime) RuntimeStatus {
	for _, name := range runtimeExecutables(rt.Name, r.GOOS) {
		if path, found := lookPath(name, r.Env, r.GOOS); found {
			return r.probeRuntime(ctx, rt, path, "PATH has")
		}
	}
	return RuntimeStatus{Runtime: rt, Detail: runtimeNeed(rt)}
}

// probeRuntime reads the version of the runtime at path against its
// bounds; where names, in the detail, what holds that executable.
func (r Resolver) probeRuntime(ctx context.Context, rt Runtime, path, where string) RuntimeStatus {
	status := RuntimeStatus{Runtime: rt, Path: path, Detail: runtimeNeed(rt)}
	out, err := r.run(ctx, runner.Command{Name: path, Args: rt.Probe, Env: ToolEnv(r.Env)})
	line := firstLine(out)
	if err == nil {
		status.Version = runtimeVersion.FindString(line)
	}
	status.OK = status.Version != "" && within(status.Version, rt)
	if !status.OK {
		status.Detail += fmt.Sprintf(": %s %s answering %s", where, path, line)
	}
	return status
}

// within compares in semver, which reads 3.10 as 3.10.0.
func within(version string, rt Runtime) bool {
	v := "v" + version
	if semver.Compare(v, "v"+rt.Floor) < 0 {
		return false
	}
	return rt.Below == "" || semver.Compare(v, "v"+rt.Below) < 0
}

// runtimeNeed is the sentence of a runtime's bounds the person reads.
func runtimeNeed(rt Runtime) string {
	label, group := "Node", "npm tools"
	if rt.Name == runtimePython {
		label, group = "Python", "PyPI tools"
	}
	need := fmt.Sprintf("the %s need %s %s or newer", group, label, trimZeros(rt.Floor))
	if rt.Below != "" {
		need += " and older than " + trimZeros(rt.Below)
	}
	return need + " on PATH"
}

// trimZeros drops trailing .0 parts, so 22.13.0 reads 22.13 and 3.10 stays.
func trimZeros(version string) string {
	for strings.Count(version, ".") > 1 {
		trimmed, cut := strings.CutSuffix(version, ".0")
		if !cut {
			break
		}
		version = trimmed
	}
	return version
}

// runtimeExecutables are the names a runtime is found as, in order;
// Windows installs python without the 3.
func runtimeExecutables(name, goos string) []string {
	if name != runtimePython {
		return []string{name}
	}
	if goos == goosWindows {
		return []string{"python", "python3"}
	}
	return []string{"python3", "python"}
}

// runtimeOf finds the runtime a tool's distribution needs.
func runtimeOf(t Tool) (Runtime, bool) {
	name := t.Runtime()
	if name == "" {
		return Runtime{}, false
	}
	for _, rt := range Runtimes() {
		if rt.Name == name {
			return rt, true
		}
	}
	return Runtime{}, false
}

// lookPath finds name on the PATH of env, never on the process's own, so
// the resolver answers for the environment the tool will run in.
func lookPath(name string, env []string, goos string) (string, bool) {
	candidates := []string{name}
	if goos == goosWindows {
		candidates = []string{name + ".exe", name + ".cmd", name + ".bat", name}
	}
	for _, dir := range filepath.SplitList(pathOf(env)) {
		if dir == "" {
			continue
		}
		for _, candidate := range candidates {
			path := filepath.Join(dir, candidate)
			if executable(path, goos) {
				return path, true
			}
		}
	}
	return "", false
}

// pathOf answers the last PATH entry of env, the one a process sees;
// Windows spells the name in any case.
func pathOf(env []string) string {
	value := ""
	for _, entry := range env {
		name, rest, found := strings.Cut(entry, "=")
		if found && strings.EqualFold(name, "PATH") {
			value = rest
		}
	}
	return value
}

func executable(path, goos string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	return goos == goosWindows || info.Mode()&executableBits != 0
}

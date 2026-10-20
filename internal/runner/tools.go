package runner

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"unicode"
)

// probeVersion is the argument most pinned tools print their version for.
const probeVersion = "version"

// Tool is one pinned program of the declared toolchain (section 5.7).
type Tool struct {
	Name    string   // the executable name, without .exe
	Module  string   // the package path go install takes
	Version string   // the exact pin, without the leading v
	Probe   []string // the arguments that make the tool print its version
}

// Toolchain lists the pinned tools of this kit version in name order; a
// kit release moves these pins, and the exact pin of A10 keeps one binary,
// and so one toolchain, per repository.
func Toolchain() []Tool {
	return []Tool{
		{Name: "gitleaks", Module: "github.com/zricethezav/gitleaks/v8", Version: "8.30.1", Probe: []string{probeVersion}},
		{Name: "golangci-lint", Module: "github.com/golangci/golangci-lint/v2/cmd/golangci-lint", Version: "2.14.0", Probe: []string{probeVersion}},
		{Name: "govulncheck", Module: "golang.org/x/vuln/cmd/govulncheck", Version: "1.7.0", Probe: []string{"-version"}},
		{Name: "lefthook", Module: "github.com/evilmartians/lefthook/v2", Version: "2.1.12", Probe: []string{probeVersion}},
	}
}

// LookupTool finds a tool of the toolchain by name.
func LookupTool(name string) (Tool, bool) {
	tools := Toolchain()
	i := slices.IndexFunc(tools, func(t Tool) bool { return t.Name == name })
	if i < 0 {
		return Tool{}, false
	}
	return tools[i], true
}

// InstallArgs are the arguments of the go command that installs the pinned
// tool: install, then Module@vVersion.
func (t Tool) InstallArgs() []string {
	return []string{"install", t.Module + "@v" + t.Version}
}

// Matches reports whether a probe output names the pinned version as a
// whole token, with or without a leading v: "version 2.14.0" and
// "govulncheck@v1.7.0" match their pins, "2.14.01" and "2.14.0-rc1" do not.
func (t Tool) Matches(output string) bool {
	tokens := strings.FieldsFunc(output, func(r rune) bool { return !versionRune(r) })
	return slices.ContainsFunc(tokens, func(token string) bool { return token == t.Version || token == "v"+t.Version })
}

// Verify finds the tool through the PATH of the process, runs its probe
// with env and answers the path it found: a tool PATH does not hold is
// ErrToolMissing, and a tool whose probe fails or names another version is
// ErrToolVersion with the first line it printed; both name the pin and the
// fix.
func Verify(ctx context.Context, t Tool, env []string) (string, error) {
	path, err := exec.LookPath(t.Name)
	if err != nil {
		return "", fmt.Errorf("%w: %s %s is not on PATH: run golaw setup", ErrToolMissing, t.Name, t.Version)
	}
	out, err := Run(ctx, Command{Name: path, Args: t.Probe, Env: env})
	if err != nil {
		return "", err
	}
	if out.Code != 0 || !t.Matches(string(out.Stdout)) {
		return "", fmt.Errorf("%w: %s at %s answers %q and the pin is %s: run golaw setup",
			ErrToolVersion, t.Name, path, firstLine(out), t.Version)
	}
	return path, nil
}

// versionRune reports whether a rune belongs to a version token.
func versionRune(r rune) bool {
	return r == '.' || r == '-' || r == '+' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// firstLine is the first line a probe printed, on stdout or else on stderr.
func firstLine(out Output) string {
	text := strings.TrimSpace(string(out.Stdout))
	if text == "" {
		text = strings.TrimSpace(string(out.Stderr))
	}
	line, _, _ := strings.Cut(text, "\n")
	return line
}

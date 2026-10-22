package assets_test

import (
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// The shape of one verified download: the digest length and the lines of
// the sequence from the download to the installation or extraction.
const (
	ciDigestLength    = 64
	ciDownloadLines   = 3
	ciChecksumPrefix  = `printf '%s  %s\n' '`
	ciChecksumSuffix  = " | sha256sum -c -"
	ciDownloadBinPath = `"$RUNNER_TEMP/golaw-bin"`
)

// ciDownloadCommand recognizes the lines of the verified download sequence
// the template emits for the image tools; any other line is a form the
// proof does not know.
func ciDownloadCommand(line string, words []string) (ciCommand, error) {
	switch {
	case line == "mkdir -p "+ciDownloadBinPath:
		return ciCommand{kind: ciKindMkdir}, nil
	case line == "echo "+ciDownloadBinPath+` >> "$GITHUB_PATH"`:
		return ciCommand{kind: ciKindAddPath}, nil
	case len(words) == 5 && words[0] == "curl" && words[1] == "-fsSL" && strings.HasPrefix(words[2], "https://") && words[3] == "-o":
		return ciCommand{kind: ciKindDownload}, nil
	case strings.HasPrefix(line, ciChecksumPrefix) && strings.HasSuffix(line, ciChecksumSuffix):
		return ciCommand{kind: ciKindChecksum}, nil
	case len(words) == 5 && words[0] == "install" && words[1] == "-m" && words[2] == "0755":
		return ciCommand{kind: ciKindInstall}, nil
	case len(words) == 6 && words[0] == "tar" && words[1] == "-xzf" && words[3] == "-C":
		return ciCommand{kind: ciKindExtract}, nil
	}
	return ciCommand{}, fmt.Errorf("%w: command form the proof does not know: %q", errCIReplay, line)
}

// ciDownloadTools names the tools the steps download: each download is
// one ordered sequence, the download, its SHA-256 verification against a
// literal digest and the installation or extraction of the same file, and
// a tool counts only once its sequence is complete.
func ciDownloadTools(steps []assets.RunStep) (map[string]bool, error) {
	tools := make(map[string]bool)
	for _, step := range steps {
		lines := ciStepLines(step)
		for index := 0; index < len(lines); {
			command, err := ciCommandOf(lines[index])
			if err != nil {
				return nil, err
			}
			switch command.kind {
			case ciKindDownload:
				tool, sequenceErr := ciDownloadSequence(lines[index:])
				if sequenceErr != nil {
					return nil, sequenceErr
				}
				tools[tool] = true
				index += ciDownloadLines
			case ciKindChecksum, ciKindInstall, ciKindExtract:
				return nil, fmt.Errorf("%w: %q outside a download sequence", errCIReplay, lines[index])
			default:
				index++
			}
		}
	}
	return tools, nil
}

// ciDownloadSequence reads one download sequence from its curl line and
// answers the tool it installs: hadolint through install, trivy through
// extraction, each on the file the download wrote and the checksum verified.
func ciDownloadSequence(lines []string) (string, error) {
	if len(lines) < ciDownloadLines {
		return "", fmt.Errorf("%w: download %q without its verification and installation", errCIReplay, lines[0])
	}
	file := strings.Fields(lines[0])[4]
	if err := ciChecksumLine(lines[1], file); err != nil {
		return "", err
	}
	words := strings.Fields(lines[2])
	command, err := ciCommandOf(lines[2])
	if err != nil {
		return "", err
	}
	var tool string
	switch {
	case command.kind == ciKindInstall && words[3] == file:
		tool = path.Base(strings.Trim(words[4], `"`))
	case command.kind == ciKindExtract && words[2] == file:
		tool = words[5]
	default:
		return "", fmt.Errorf("%w: download of %s is followed by %q, not its installation", errCIReplay, file, lines[2])
	}
	if tool != "hadolint" && tool != "trivy" {
		return "", fmt.Errorf("%w: download of %s installs %s, which the proof does not know", errCIReplay, file, tool)
	}
	return tool, nil
}

// ciChecksumLine requires the verification of a downloaded file against a
// literal lowercase SHA-256 digest, piped into sha256sum -c -.
func ciChecksumLine(line, file string) error {
	body, ok := strings.CutPrefix(line, ciChecksumPrefix)
	if !ok || !strings.HasSuffix(body, ciChecksumSuffix) {
		return fmt.Errorf("%w: download of %s is followed by %q, not its checksum", errCIReplay, file, line)
	}
	digest, verified, ok := strings.Cut(strings.TrimSuffix(body, ciChecksumSuffix), "' ")
	if !ok || verified != file || len(digest) != ciDigestLength || strings.Trim(digest, "0123456789abcdef") != "" {
		return fmt.Errorf("%w: checksum line %q does not verify %s against a literal digest", errCIReplay, line, file)
	}
	return nil
}

// ciInstalledTools names the tools the steps of a job provision: the exact
// Go install, npm install and verified download forms, and the Docker the
// hosted runner supplies, which the step docker version checks.
func ciInstalledTools(steps []assets.RunStep) (map[string]bool, error) {
	tools, err := ciDownloadTools(steps)
	if err != nil {
		return nil, err
	}
	for _, step := range steps {
		for _, line := range ciStepLines(step) {
			tool, lineErr := ciInstalledLine(line)
			if lineErr != nil {
				return nil, lineErr
			}
			if tool != "" {
				tools[tool] = true
			}
		}
	}
	return tools, nil
}

// ciInstalledLine names the tool one line provisions, nothing for a line
// that installs nothing.
func ciInstalledLine(line string) (string, error) {
	command, err := ciCommandOf(line)
	if err != nil {
		return "", err
	}
	words := strings.Fields(line)
	switch command.kind {
	case ciKindGo:
		module, _, _ := strings.Cut(words[len(words)-1], "@")
		return ciModuleTool(module)
	case ciKindNpm:
		return ciNpmTool(words[3][:strings.LastIndex(words[3], "@")])
	case ciKindDocker:
		return "docker", nil
	}
	return "", nil
}

// ciModuleTool maps the Go modules the template installs to the tool name
// a recipe runs.
func ciModuleTool(module string) (string, error) {
	tools := map[string]string{
		"github.com/cristianargotti/golaw/cmd/golaw":             "golaw",
		"github.com/golangci/golangci-lint/v2/cmd/golangci-lint": "golangci-lint",
		"github.com/zricethezav/gitleaks/v8":                     "gitleaks",
		"golang.org/x/vuln/cmd/govulncheck":                      "govulncheck",
	}
	if tool, ok := tools[module]; ok {
		return tool, nil
	}
	return "", fmt.Errorf("%w: go install of %s provisions no tool the proof knows", errCIReplay, module)
}

// ciNpmTool maps the npm packages the template installs to the tool name
// a recipe runs.
func ciNpmTool(pkg string) (string, error) {
	tools := map[string]string{"@redocly/cli": "redocly", "squawk-cli": "squawk"}
	if tool, ok := tools[pkg]; ok {
		return tool, nil
	}
	return "", fmt.Errorf("%w: npm install of %s provisions no tool the proof knows", errCIReplay, pkg)
}

// ciInstallCalls requires every go install and npm install line of the job
// to have reached its fake.
func ciInstallCalls(t *testing.T, replay ciJobReplay, job ciJobPlan) error {
	t.Helper()
	for _, step := range job.steps {
		for _, line := range ciStepLines(step) {
			words := strings.Fields(line)
			if len(words) < 2 || words[1] != "install" || (words[0] != "go" && words[0] != "npm") {
				continue
			}
			calls := runnertest.Calls(t, replay.bin, words[0])
			if !slices.ContainsFunc(calls, func(call []string) bool { return slices.Equal(call, words[1:]) }) {
				return fmt.Errorf("%w: job %s never reached %q: %v", errCIReplay, job.id, line, calls)
			}
		}
	}
	return nil
}

// ciDownloadCalls requires each image download of the job verified with
// sha256sum -c -, both tools installed or extracted once, and the hosted
// Docker checked first and built only by a job whose recipes run it.
func ciDownloadCalls(t *testing.T, replay ciJobReplay, job ciJobPlan) error {
	t.Helper()
	checks := runnertest.Calls(t, replay.bin, "sha256sum")
	if !slices.EqualFunc(checks, [][]string{{"-c", "-"}, {"-c", "-"}}, slices.Equal) {
		return fmt.Errorf("%w: job %s verified %v, want two sha256sum -c -", errCIReplay, job.id, checks)
	}
	docker := runnertest.Calls(t, replay.bin, "docker")
	if len(docker) == 0 || !slices.Equal(docker[0], []string{"version"}) {
		return fmt.Errorf("%w: job %s does not check Docker first: %v", errCIReplay, job.id, docker)
	}
	want := map[string]int{"curl": 2, "install": 1, "tar": 1, "docker": 1}
	if job.consumed["docker"] {
		want["docker"] = 2
	}
	for _, tool := range slices.Sorted(maps.Keys(want)) {
		if got := len(runnertest.Calls(t, replay.bin, tool)); got != want[tool] {
			return fmt.Errorf("%w: job %s ran %s %d times, want %d", errCIReplay, job.id, tool, got, want[tool])
		}
	}
	return nil
}

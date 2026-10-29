package assets_test

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runnertest"
)

// The kinds of command line the CI template emits in its run steps.
const (
	ciKindMake     = "make"
	ciKindGitDiff  = "git-diff"
	ciKindRestore  = "restore"
	ciKindGo       = "go-install"
	ciKindNpm      = "npm-install"
	ciKindDocker   = "docker"
	ciKindMkdir    = "mkdir"
	ciKindAddPath  = "add-path"
	ciKindDownload = "download"
	ciKindChecksum = "checksum"
	ciKindInstall  = "install"
	ciKindExtract  = "extract"
	ciKindSurface  = "surface-tools"
)

// ciFailureCode is the exit status of the fake tool whose failure a replay
// injects, distinct from the codes the real tools answer.
const ciFailureCode = 9

// ciVulngateModule is the kit tool whose pinned run needs govulncheck.
const ciVulngateModule = "github.com/cristianargotti/golaw/tools/vulngate"

// ciCommand is one recognized command line of a run step: its kind and,
// for a make line, the targets it names.
type ciCommand struct {
	kind    string
	targets []string
}

// ciFakeTools names every program the replay stands in for; bash, make and
// the shell builtins stay real.
func ciFakeTools() []string {
	return []string{
		"go", "golaw", "golangci-lint", "gitleaks", "govulncheck", "git", "npm", "squawk",
		"redocly", "hadolint", "trivy", "docker", "curl", "sha256sum", "tar", "install",
	}
}

// ciCommandOf recognizes one command line of a run step by the exact forms
// the CI template emits and answers its kind, with the targets of a make line.
func ciCommandOf(line string) (ciCommand, error) {
	words := strings.Fields(line)
	switch {
	case len(words) == 0:
		return ciCommand{}, fmt.Errorf("%w: empty command", errCIReplay)
	case words[0] == "make":
		targets, err := ciMakeTargets(words[1:])
		return ciCommand{kind: ciKindMake, targets: targets}, err
	case line == "git diff --exit-code":
		return ciCommand{kind: ciKindGitDiff}, nil
	case line == `echo "GOTOOLCHAIN=auto" >> "$GITHUB_ENV"`:
		return ciCommand{kind: ciKindRestore}, nil
	case words[0] == "go" && len(words) > 2 && words[1] == "install" && ciGoInstallForm(words[2:]):
		return ciCommand{kind: ciKindGo}, nil
	case len(words) == 4 && words[0] == "npm" && words[1] == "install" && words[2] == "--global" && strings.LastIndex(words[3], "@") > 0:
		return ciCommand{kind: ciKindNpm}, nil
	case line == "docker version":
		return ciCommand{kind: ciKindDocker}, nil
	case line == "golaw setup --surface":
		return ciCommand{kind: ciKindSurface}, nil
	}
	return ciDownloadCommand(line, words)
}

// ciGoInstallForm accepts the module@version of a go install, alone or
// after the one -ldflags word the gitleaks installation carries.
func ciGoInstallForm(words []string) bool {
	if len(words) == 2 && strings.HasPrefix(words[0], "-ldflags=") {
		words = words[1:]
	}
	return len(words) == 1 && strings.Contains(words[0], "@v")
}

// ciMakeTargets accepts the literal target lists the template emits.
func ciMakeTargets(words []string) ([]string, error) {
	if len(words) == 0 {
		return nil, fmt.Errorf("%w: make without a target", errCIReplay)
	}
	for _, word := range words {
		if strings.Trim(word, "abcdefghijklmnopqrstuvwxyz0123456789-") != "" {
			return nil, fmt.Errorf("%w: make target %q is not a literal name", errCIReplay, word)
		}
	}
	return words, nil
}

// ciRecipeTools names the check tools the expanded recipe lines run: a
// plain program, the tool golaw lock runs, and govulncheck behind the
// pinned vulngate scan; go and git are execution prerequisites.
func ciRecipeTools(commands []string) (map[string]bool, error) {
	tools := make(map[string]bool)
	for _, command := range commands {
		words := ciRecipeWords(command)
		if len(words) == 0 {
			return nil, fmt.Errorf("%w: empty recipe line %q", errCIReplay, command)
		}
		named, err := ciRecipeTool(words)
		if err != nil {
			return nil, err
		}
		for _, tool := range named {
			tools[tool] = true
		}
	}
	return tools, nil
}

// ciRecipeWords strips the make prefixes and the literal assignments a
// recipe line may start with, so the program is the first word.
func ciRecipeWords(command string) []string {
	words := strings.Fields(strings.TrimLeft(command, "@-+ \t"))
	for len(words) != 0 {
		name, _, assignment := strings.Cut(words[0], "=")
		if !assignment || strings.Trim(name, "ABCDEFGHIJKLMNOPQRSTUVWXYZ_") != "" {
			break
		}
		words = words[1:]
	}
	return words
}

// ciRecipeTool names the check tools one recipe line requires.
func ciRecipeTool(words []string) ([]string, error) {
	switch words[0] {
	case "go":
		if len(words) > 2 && words[1] == "run" && strings.HasPrefix(words[2], ciVulngateModule+"@v") {
			return []string{"govulncheck"}, nil
		}
		return nil, nil
	case "git":
		return nil, nil
	case "golaw":
		if len(words) > 1 && words[1] == "lint" && slices.Contains(words, "dockerfile") {
			return []string{"golaw", "hadolint"}, nil
		}
		if len(words) > 1 && words[1] == "lock" {
			separator := slices.Index(words, "--")
			if separator < 0 || separator+1 >= len(words) {
				return nil, fmt.Errorf("%w: golaw lock without the command it runs: %q", errCIReplay, strings.Join(words, " "))
			}
			return []string{"golaw", words[separator+1]}, nil
		}
		return []string{"golaw"}, nil
	case "golangci-lint", "gitleaks", "govulncheck", "redocly", "squawk", "hadolint", "trivy", "docker":
		return []string{words[0]}, nil
	}
	return nil, fmt.Errorf("%w: recipe runs %s, which the proof does not know", errCIReplay, words[0])
}

// ciExpectedTools requires every installed tool to have a recipe that runs
// it and the union of the installations to be exactly the tools of the
// capability set, so an optional consumer disappears with its capability.
func ciExpectedTools(jobs []ciJobPlan, caps []string) error {
	installed := make(map[string]bool)
	consumed := make(map[string]bool)
	for _, job := range jobs {
		maps.Copy(installed, job.installed)
		maps.Copy(consumed, job.consumed)
	}
	for _, tool := range slices.Sorted(maps.Keys(installed)) {
		if !consumed[tool] {
			return fmt.Errorf("%w: %s is installed and no recipe runs it", errCIReplay, tool)
		}
	}
	want := []string{"golaw"}
	for capability, tools := range map[string][]string{
		"go": {"golangci-lint", "gitleaks", "govulncheck"}, "api": {"redocly"}, "db": {"squawk"},
		"image": {"hadolint", "trivy", "docker"},
	} {
		if slices.Contains(caps, capability) {
			want = append(want, tools...)
		}
	}
	slices.Sort(want)
	if got := slices.Sorted(maps.Keys(installed)); !slices.Equal(got, want) {
		return fmt.Errorf("%w: installed %v, want %v for %v", errCIReplay, got, want, caps)
	}
	return nil
}

// ciReplayTools installs the fakes of every program the replay stands in
// for, each answering a version to a nonempty probe and code 0, except the
// one whose failure the replay injects; it answers each fake's path. The
// fakes after the first are hard links to it where the file system allows,
// because a job would otherwise write sixteen copies of the test binary.
func ciReplayTools(t *testing.T, bin, fail string) map[string]string {
	t.Helper()
	tools := make(map[string]string, len(ciFakeTools()))
	first := ""
	for _, name := range ciFakeTools() {
		behavior := runnertest.Behavior{Probe: []string{"--version"}, Version: name + " fake"}
		if name == fail {
			behavior.Code = ciFailureCode
		}
		if first != "" {
			if err := os.Link(first, filepath.Join(bin, name+filepath.Ext(first))); err != nil {
				t.Logf("the fake %s is a copy, not a link: %v", name, err)
			}
		}
		tools[name] = runnertest.Install(t, bin, name, behavior)
		if first == "" {
			first = tools[name]
		}
	}
	return tools
}

// ciScanCalls requires the gate's golaw scan with the enrollment the
// pipeline supplied, and the plain scan of image-pins-check beside it when
// the image targets run.
func ciScanCalls(t *testing.T, replay ciJobReplay, in ciReplayInput) error {
	t.Helper()
	calls := runnertest.Calls(t, replay.bin, "golaw")
	want := []string{"scan", "."}
	if in.Enrollment != "" {
		want = []string{"scan", "--enrolled", in.Enrollment, "."}
	}
	contains := func(args []string) bool {
		return slices.ContainsFunc(calls, func(call []string) bool { return slices.Equal(call, args) })
	}
	if !contains(want) || (slices.Contains(in.Caps, "image") && !contains([]string{"scan", "."})) {
		return fmt.Errorf("%w: golaw calls %v lack the scan %v", errCIReplay, calls, want)
	}
	return nil
}

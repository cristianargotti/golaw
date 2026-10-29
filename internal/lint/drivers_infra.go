package lint

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/surface"
)

// The infrastructure tools: checkov and hadolint hold parity rules, the
// others only strict ones.
const (
	infraCfnLint    = "cfn-lint"
	infraCheckov    = "checkov"
	infraHadolint   = "hadolint"
	infraShellcheck = "shellcheck"
	infraShfmt      = "shfmt"
	infraYamllint   = "yamllint"
)

// infraShebangBytes bounds the read of a first line: Linux reads no more of
// an interpreter line, so the bytes past it name no interpreter there.
const infraShebangBytes = 256

// Each literal would otherwise repeat three times, which goconst and the
// kit's own literal check refuse.
const (
	infraFlagConfigFile = "--config-file"
	infraFlagFormat     = "--format"
	infraJSON           = "json"
	infraStdin          = "-"
)

// infraRuns answers whether a tool has anything to run: a file of its
// languages, and a set that holds its rules (inParity names a tool whose
// rules the parity set holds); an unknown set starts no strict-only tool.
func infraRuns(in Input, inParity bool) bool {
	return len(in.Files) > 0 && (inParity || in.Set == model.SurfaceStrict)
}

// infraPaths answers the files as absolute paths, so a result never depends
// on the directory a tool starts in.
func infraPaths(root string, files []string) []string {
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, filepath.Join(root, filepath.FromSlash(f)))
	}
	return paths
}

// infraDrivers answers the drivers of the infrastructure surface, sorted by
// tool.
func infraDrivers() []Driver {
	return []Driver{cfnLintDriver(), checkovDriver(), hadolintDriver(), shellcheckDriver(), shfmtDriver(), yamllintDriver()}
}

// hadolintDriver lints Dockerfiles in both sets and in the hook.
func hadolintDriver() Driver {
	return Driver{
		Tool: infraHadolint, Languages: []surface.Language{surface.Dockerfile}, Format: FormatHadolint,
		Args: hadolintArgs, HookArgs: hadolintHookArgs, Config: infraConfig(),
	}
}

// hadolintArgs runs the set's file over every Dockerfile of the scope.
func hadolintArgs(in Input) []string {
	if !infraRuns(in, true) {
		return nil
	}
	return append(hadolintFlags(in.ConfigDir), infraPaths(in.Root, in.Files)...)
}

// hadolintHookArgs reads the written bytes on stdin.
func hadolintHookArgs(in HookInput) []string {
	return append(hadolintFlags(in.ConfigDir), infraStdin)
}

// hadolintFlags passes the kit file, which replaces any .hadolint.yaml the
// repository or the home holds; no --ignore or --error flag, because a
// command line rule would drop the ignored list of the file.
func hadolintFlags(configDir string) []string {
	return []string{configFlag, filepath.Join(configDir, hadolintConfig), infraFlagFormat, infraJSON, "--no-color"}
}

// checkovDriver checks Dockerfiles and CloudFormation templates.
func checkovDriver() Driver {
	return Driver{
		Tool: infraCheckov, Languages: []surface.Language{surface.Dockerfile, surface.CloudFormation},
		Format: FormatCheckov, Args: checkovArgs, Config: infraConfig(),
	}
}

// checkovArgs keeps checkov offline and, under parity, whose checks are all
// CloudFormation ones, leaves the Dockerfiles out. --skip-download stands on
// the command line too, so no configuration merge turns the download on.
func checkovArgs(in Input) []string {
	files := in.Files
	if in.Set != model.SurfaceStrict {
		files = slices.DeleteFunc(slices.Clone(files), isDockerfile)
	}
	if len(files) == 0 {
		return nil
	}
	args := []string{infraFlagConfigFile, filepath.Join(in.ConfigDir, checkovConfig), "--output", infraJSON, "--quiet", "--compact", "--skip-download"}
	for _, path := range infraPaths(in.Root, files) {
		args = append(args, "--file", path)
	}
	return args
}

// isDockerfile answers whether a path is a Dockerfile by its name alone.
func isDockerfile(path string) bool {
	language, ok := surface.ByName(path)
	return ok && language == surface.Dockerfile
}

// shellcheckDriver lints shell scripts. Sonar way holds no shell rule, so
// it runs in the strict set and in the hook, where a finding is feedback.
func shellcheckDriver() Driver {
	return Driver{
		Tool: infraShellcheck, Languages: []surface.Language{surface.Shell}, Format: FormatShellcheck,
		Args: shellcheckArgs, HookArgs: shellcheckHookArgs, Config: infraConfig(),
	}
}

// shellcheckArgs ignores every .shellcheckrc and follows a sourced file from
// the script's own directory.
func shellcheckArgs(in Input) []string {
	paths := shellPaths(in.Root, in.Files)
	if len(paths) == 0 || !infraRuns(in, false) {
		return nil
	}
	return append([]string{"--norc", "--format=json1", "--severity=style", "--external-sources", "--source-path=SCRIPTDIR"}, paths...)
}

// shellcheckHookArgs reads the written bytes on stdin and follows no sourced
// file, which keeps the hook inside its budget.
func shellcheckHookArgs(in HookInput) []string {
	args := []string{"--norc", "--format=json1"}
	if in.Shell != "" {
		args = append(args, "--shell="+in.Shell)
	}
	return append(args, infraStdin)
}

// shfmtDriver reports a script whose layout differs from the strict style.
func shfmtDriver() Driver {
	return Driver{Tool: infraShfmt, Languages: []surface.Language{surface.Shell}, Format: FormatShfmt, Args: shfmtArgs, Config: infraConfig()}
}

// shfmtArgs passes printer flags, which also make shfmt ignore every
// .editorconfig of the repository.
func shfmtArgs(in Input) []string {
	paths := shellPaths(in.Root, in.Files)
	if len(paths) == 0 || !infraRuns(in, false) {
		return nil
	}
	return append([]string{"-d", "-i", "2", "-ci"}, paths...)
}

// shellPaths answers infraPaths without the zsh scripts, which neither
// shellcheck nor shfmt parses (the hook skips them the same way); a file
// whose first line cannot be read stays, so the tool names it.
func shellPaths(root string, files []string) []string {
	kept := make([]string, 0, len(files))
	for _, f := range files {
		if !zshScript(filepath.Join(root, filepath.FromSlash(f))) {
			kept = append(kept, f)
		}
	}
	return infraPaths(root, kept)
}

// zshScript reads the first line: a #! line whose interpreter, or the first
// word after env and its flags, is zsh.
func zshScript(path string) bool {
	file, err := os.Open(path) //nolint:gosec // a file of the analyzer scope the lint target reads anyway
	if err != nil {
		return false
	}
	defer file.Close()
	line, err := bufio.NewReader(io.LimitReader(file, infraShebangBytes)).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	words := strings.Fields(strings.TrimPrefix(line, "#!"))
	if !strings.HasPrefix(line, "#!") || len(words) == 0 {
		return false
	}
	interpreter := filepath.Base(words[0])
	for _, word := range words[1:] {
		if interpreter != "env" {
			break
		}
		if !strings.HasPrefix(word, "-") {
			interpreter = word
		}
	}
	return interpreter == "zsh"
}

// cfnLintDriver lints CloudFormation templates at cfn-lint's defaults.
func cfnLintDriver() Driver {
	return Driver{Tool: infraCfnLint, Languages: []surface.Language{surface.CloudFormation}, Format: FormatCfnLint, Args: cfnLintArgs, Config: infraConfig()}
}

// cfnLintArgs passes the kit file, which replaces a .cfnlintrc of the
// directory cfn-lint starts in.
func cfnLintArgs(in Input) []string {
	if !infraRuns(in, false) {
		return nil
	}
	return append([]string{infraFlagFormat, infraJSON, infraFlagConfigFile, filepath.Join(in.ConfigDir, cfnLintConfig)}, infraPaths(in.Root, in.Files)...)
}

// yamllintDriver lints plain YAML, CloudFormation excluded by detection.
func yamllintDriver() Driver {
	return Driver{Tool: infraYamllint, Languages: []surface.Language{surface.YAML}, Format: FormatYamllint, Args: yamllintArgs, Config: infraConfig()}
}

// yamllintArgs passes the kit file, which replaces a .yamllint of the
// repository, and fails on warnings as the strict set asks.
func yamllintArgs(in Input) []string {
	if !infraRuns(in, false) {
		return nil
	}
	return append([]string{infraFlagConfigFile, filepath.Join(in.ConfigDir, yamllintConfig), infraFlagFormat, "parsable", "--strict"}, infraPaths(in.Root, in.Files)...)
}

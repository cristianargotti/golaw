package goldentest

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"os"
	pathpkg "path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/genheader"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/source"
)

// tailLines is how much of a failed tool's output a failure carries.
const tailLines = 40

// lintedHeader stands in for the golaw header while the linters run:
// revive skips a file that carries the Go generated convention whatever
// linters.exclusions.generated says, and the pipeline analyses it.
const lintedHeader = "// golaw regenerable file, linted as hand written code."

// pipelineProfile is the cover profile of the pipeline shape.
const pipelineProfile = "profile.cov"

// The arguments every go test of the harness shares.
const (
	goTest       = "test"
	goCount      = "-count=1"
	everyPackage = "./..."
)

// pipelineTest is go test in the pipeline shape: a cover profile and no
// -coverpkg, so each package credits only its own statements (M7.5).
func pipelineTest(profile string) []string {
	return []string{goTest, goCount, "-coverprofile=" + profile, everyPackage}
}

// build compiles, vets and tests the module in the pipeline shape, which
// writes the profile the coverage step reads, and with the race detector.
func (h *harness) build(ctx context.Context) {
	profile := filepath.Join(h.work, pipelineProfile)
	runs := [][]string{
		{"build", everyPackage},
		{"vet", everyPackage},
		pipelineTest(profile),
		{goTest, goCount, "-race", everyPackage},
	}
	for _, args := range runs {
		if !h.goStep(ctx, StepBuild, args...) {
			return
		}
	}
	h.profile = profile
}

// goStep runs one go command and fails the step on anything but exit 0.
func (h *harness) goStep(ctx context.Context, step Step, args ...string) bool {
	out, err := goCommand(ctx, h.repo.Root, args...)
	if err != nil {
		h.failf(string(step), "%v", err)
		return false
	}
	if out.Code != 0 {
		h.failf(string(step), "go %s: exit %d: %s", strings.Join(args, " "), out.Code, tail(out))
		return false
	}
	return true
}

// lintSetup resolves golangci-lint and the configuration of a lint or format
// step; an empty path means the step was skipped or failed already.
func (h *harness) lintSetup(ctx context.Context, step Step) (string, string) {
	resolve := h.resolveLint
	if resolve == nil {
		resolve = resolveLint
	}
	path, reason := resolve(ctx, h.repo.Root)
	if reason != "" {
		h.skip(step, reason)
		return "", ""
	}
	config, err := h.lintConfig(false)
	if err != nil {
		h.failf(string(step), "%v", err)
		return "", ""
	}
	return path, config
}

// lint runs the pinned golangci-lint with the rendered asset over the
// packages the commands wrote into, every golaw header stood in for while
// it runs; the base's own packages are the contracts, not the slice.
func (h *harness) lint(ctx context.Context) {
	tool, config := h.lintSetup(ctx, StepLint)
	if tool == "" {
		return
	}
	restore, err := h.unheader()
	if err == nil {
		h.lintRun(ctx, tool, config, "")
	}
	// The headers come back before the adopter's run, which must see them.
	restore()
	if err != nil {
		h.failf(string(StepLint), "%v", err)
		return
	}
	h.lintAsAdopted(ctx, tool)
}

// lintAsAdopted runs the rendered asset as the adopter's make lint does,
// generated: lax and the golaw headers kept, so an owned file that lacks
// what only a regenerable file holds, such as the package comment, fails
// here as it fails there.
func (h *harness) lintAsAdopted(ctx context.Context, tool string) {
	config, err := h.lintConfig(true)
	if err != nil {
		h.failf(string(StepLint), "%v", err)
		return
	}
	h.lintRun(ctx, tool, config, "as the adopter runs it: ")
}

// lintRun runs golangci-lint over the written packages with one config.
func (h *harness) lintRun(ctx context.Context, tool, config, label string) {
	out, err := runner.Run(ctx, runner.Command{
		Name: tool, Args: append([]string{"run", "--allow-parallel-runners", "-c", config}, h.packages()...), Dir: h.repo.Root,
	})
	switch {
	case err != nil:
		h.failf(string(StepLint), "%s%v", label, err)
	case out.Code != 0:
		h.failf(string(StepLint), "%sexit %d: %s", label, out.Code, tail(out))
	}
}

// unheader replaces the header of every written regenerable Go file and
// answers the function that puts the bytes back, whatever happened.
func (h *harness) unheader() (func(), error) {
	saved := map[string][]byte{}
	restore := func() {
		for path, content := range saved {
			if err := os.WriteFile(path, content, fillMode); err != nil {
				h.failf(string(StepLint), "restore %s: %v", path, err)
			}
		}
	}
	for _, rel := range h.headed() {
		path := source.OSPath(h.repo.Root, rel)
		content, err := os.ReadFile(path) //nolint:gosec // the path is one the commands wrote
		if err != nil {
			return restore, fmt.Errorf(readFailed, rel, err)
		}
		_, rest, _ := bytes.Cut(content, []byte("\n"))
		saved[path] = content
		if err := os.WriteFile(path, slices.Concat([]byte(lintedHeader+"\n"), rest), fillMode); err != nil { //nolint:gosec // the path is one the commands wrote
			return restore, fmt.Errorf("write %s: %w", rel, err)
		}
	}
	return restore, nil
}

// packages answers the package patterns of the written Go files, sorted.
func (h *harness) packages() []string {
	dirs := map[string]bool{}
	for path := range h.report.Files {
		if strings.HasSuffix(path, goSuffix) {
			dirs["./"+filepath.FromSlash(pathpkg.Dir(path))] = true
		}
	}
	return slices.Sorted(maps.Keys(dirs))
}

// headed answers the written Go files whose first line is a golaw header,
// tests included, in path order.
func (h *harness) headed() []string {
	var out []string
	for _, path := range slices.Sorted(maps.Keys(h.report.Files)) {
		if _, ok := genheader.Parse(h.report.Files[path]); ok && strings.HasSuffix(path, goSuffix) {
			out = append(out, path)
		}
	}
	return out
}

// format proves the regenerable files are byte identical to what the
// pinned formatter writes.
func (h *harness) format(ctx context.Context) {
	tool, config := h.lintSetup(ctx, StepFormat)
	if tool == "" {
		return
	}
	for _, rel := range h.headed() {
		out, err := runner.Run(ctx, runner.Command{
			Name: tool, Args: []string{"fmt", "--diff", "-c", config, filepath.FromSlash(rel)}, Dir: h.repo.Root,
		})
		if err != nil {
			h.failf(string(StepFormat), "%s differs from golangci-lint fmt: %v", rel, err)
			continue
		}
		if out.Code != 0 || len(out.Stdout) != 0 {
			h.failf(string(StepFormat), "%s differs from golangci-lint fmt: %s", rel, tail(out))
		}
	}
}

// tail answers the last lines of a tool's output, stdout first.
func tail(out runner.Output) string {
	text := strings.TrimSpace(string(out.Stdout) + "\n" + string(out.Stderr))
	lines := strings.Split(text, "\n")
	return strings.Join(lines[max(0, len(lines)-tailLines):], "\n")
}

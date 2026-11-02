package goldentest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/source"
)

// The adopter floor of A9: a template may call only what this release of
// the standard library holds, whatever toolchain the kit itself builds on.
const (
	floorGo        = "1.24"
	floorToolchain = "go1.24.0"
)

// lowered answers a go.mod whose go line names the floor, every other line
// as it was, so the requirements stay what the slice resolved.
func lowered(gomod []byte) []byte {
	lines := strings.Split(string(gomod), "\n")
	for i, line := range lines {
		if fields := strings.Fields(line); len(fields) == 2 && fields[0] == "go" {
			lines[i] = "go " + floorGo
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

// floorBinary answers the go command of the floor release, from the
// toolchain module the go command itself downloads when it switches
// releases, or why the step skips. It is run by path with the inherited
// environment, so no environment is read here; a GOTOOLCHAIN that forces
// another release shows in its version and skips the step too.
func floorBinary(ctx context.Context, work string) (string, string) {
	module := "golang.org/toolchain@v0.0.1-" + floorToolchain + "." + runtime.GOOS + "-" + runtime.GOARCH
	out, err := runner.Run(ctx, runner.Command{Name: "go", Args: []string{"mod", "download", "-json", module}, Dir: work})
	var downloaded struct {
		Dir string `json:"Dir"`
	}
	if err != nil || out.Code != 0 || json.Unmarshal(out.Stdout, &downloaded) != nil || downloaded.Dir == "" {
		return "", fmt.Sprintf("the %s toolchain is not reachable: %s", floorToolchain, cause(out, err))
	}
	bin := filepath.Join(downloaded.Dir, "bin", "go")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	version, err := runner.Run(ctx, runner.Command{Name: bin, Args: []string{"version"}, Dir: work})
	if err != nil || !bytes.Contains(version.Stdout, []byte(floorToolchain+" ")) {
		return "", fmt.Sprintf("the %s toolchain does not run as itself here: %s", floorToolchain, cause(version, err))
	}
	return bin, ""
}

// floor vets the filled slice with the floor release's go command and the
// go line of the module lowered to the floor, the way a pipeline image at
// the floor builds the adopter's code, and puts the go.mod back afterwards.
func (h *harness) floor(ctx context.Context) {
	path := source.OSPath(h.repo.Root, goModFile)
	original, err := os.ReadFile(path) //nolint:gosec // the go.mod of the scratch module
	if err != nil {
		h.failf(string(StepFloor), "read %s: %v", goModFile, err)
		return
	}
	resolve := h.floorBinary
	if resolve == nil {
		resolve = floorBinary
	}
	bin, reason := resolve(ctx, h.work)
	if reason != "" {
		h.skip(StepFloor, reason)
		return
	}
	defer h.writeGoMod(path, "restore", original)
	if h.writeGoMod(path, "write", lowered(original)) {
		out, vetErr := runner.Run(ctx, runner.Command{Name: bin, Args: []string{"vet", everyPackage}, Dir: h.repo.Root})
		if vetErr != nil || out.Code != 0 {
			h.failf(string(StepFloor), "go vet ./... at go %s: exit %d: %s", floorGo, out.Code, cause(out, vetErr))
		}
	}
}

// cause is why a go command failed: the error that kept it from running, or
// the tail of what it printed.
func cause(out runner.Output, err error) string {
	if err != nil {
		return err.Error()
	}
	return strings.TrimSpace(tail(out))
}

// writeGoMod writes the go.mod of the scratch module and answers whether it
// did; a failure is the step's, named by what the write was for.
func (h *harness) writeGoMod(path, what string, content []byte) bool {
	if err := os.WriteFile(path, content, fillMode); err != nil { //nolint:gosec // the go.mod of the scratch module
		h.failf(string(StepFloor), "%s %s: %v", what, goModFile, err)
		return false
	}
	return true
}

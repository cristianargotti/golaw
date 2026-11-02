package thirdparty

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
)

// ToolsModule is the module of the enrolled repository that pins the
// generators (M7.11.2): go tool resolves them from it, so the kit's own
// module never imports a generator.
const ToolsModule = "tools/go.mod"

// maxOutputLines bounds the generator output a failure quotes, as the hook
// bounds the lines of a file, so a broken contract never floods a reason.
const maxOutputLines = 20

// Resolve asks go tool, from the repository root, for the binary of the
// generator the tools module pins; every way it cannot answer one is
// ErrMissing naming the generator and the cause.
func Resolve(ctx context.Context, goBin, root string, g Generator, env []string) (string, error) {
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(ToolsModule))); err != nil {
		return "", fmt.Errorf("%w: %s: %s is absent: run golaw init --apply", ErrMissing, g.Name, ToolsModule)
	}
	out, err := runner.Run(ctx, runner.Command{
		Name: goBin, Args: []string{"tool", "-modfile=" + ToolsModule, "-n", g.Name}, Dir: root, Env: env,
	})
	if err != nil {
		return "", fmt.Errorf("%w: %s: go tool: %w", ErrMissing, g.Name, err)
	}
	if out.Code != 0 {
		return "", toolFailure(g.Name, out)
	}
	first, _, _ := strings.Cut(string(out.Stdout), "\n")
	bin := strings.TrimSpace(first)
	if bin == "" {
		return "", fmt.Errorf("%w: %s: go tool printed no binary", ErrMissing, g.Name)
	}
	return bin, nil
}

// incomplete are the go messages of a tools module whose requirements or
// sums init did not write: the go command's own hint, go mod download at
// the repository root, writes no tools/go.sum, so the reason names the
// command that completes the module instead.
func incomplete() []string {
	return []string{"missing go.sum entry", "updates to go.mod needed"}
}

// toolFailure is the ErrMissing of a go tool that exited with an error.
func toolFailure(name string, out runner.Output) error {
	text := strings.TrimSpace(string(out.Stderr))
	for _, message := range incomplete() {
		if strings.Contains(text, message) {
			first, _, _ := strings.Cut(text, "\n")
			first, _, _ = strings.Cut(first, "; to ")
			return fmt.Errorf("%w: %s: %s: run go -C tools mod tidy and commit tools/go.mod and tools/go.sum", ErrMissing, name, first)
		}
	}
	return fmt.Errorf("%w: %s: go tool exited %d: %s", ErrMissing, name, out.Code, text)
}

// Exec runs the generator's binary with its arguments in dir: the
// repository root for golaw gen, a stage for golaw gen --check. The output
// is answered with every error, so a caller can show what the run printed.
func Exec(ctx context.Context, bin, dir string, g Generator, env []string) (runner.Output, error) {
	out, err := runner.Run(ctx, runner.Command{Name: bin, Args: g.Args, Dir: dir, Env: env})
	if errors.Is(err, runner.ErrStart) {
		return out, fmt.Errorf("%w: %s: %w", ErrMissing, g.Name, err)
	}
	if err != nil {
		return out, fmt.Errorf("run %s: %w", g.Name, err)
	}
	if out.Code != 0 {
		return out, fmt.Errorf("%w: %s exited %d: %s", ErrFailed, g.Name, out.Code, quote(out))
	}
	return out, nil
}

// quote is what a failed generator printed, its errors first because they
// name the contract line, at most maxOutputLines lines and the count of
// the rest.
func quote(out runner.Output) string {
	text := strings.TrimSpace(strings.TrimSpace(string(out.Stderr)) + "\n" + strings.TrimSpace(string(out.Stdout)))
	lines := strings.Split(text, "\n")
	if len(lines) > maxOutputLines {
		lines = append(lines[:maxOutputLines], fmt.Sprintf("... and %d more lines", len(lines)-maxOutputLines))
	}
	return strings.Join(lines, "\n")
}

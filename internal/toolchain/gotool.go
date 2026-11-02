package toolchain

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
)

// GoBuild is one run of golaw setup --gen: the tools the enrolled
// repository's tools module pins, built with the person's go command.
type GoBuild struct {
	Dir    string // policy.Dirs.Tools; empty is ErrNoToolsDir
	Root   string // the repository root, where tools/go.mod resolves
	GOOS   string // the target the layout names
	GOARCH string // the target the layout names
	Tools  []runner.Tool
	// Run starts the go command; nil runs it through runner.Run.
	Run func(context.Context, runner.Command) (runner.Output, error)
	// Env is the person's whole environment, so the module proxy and the
	// cache settings of the machine reach the build.
	Env []string
	Out io.Writer // the pt-BR lines; nil discards them
}

// GoTool is the binary layout row of a tool the tools module pins, so the
// resolver, the receipt and the hook name it as they name a downloaded one.
func GoTool(t runner.Tool) Tool {
	return Tool{Name: t.Name, Version: t.Version, Distribution: DistBinary, Probe: t.Probe}
}

// BuildGo builds each tool in order and answers the receipt of each one
// built or already in place. It stops at the first failure, because a
// person runs setup and the refusal names the tool to look at.
func BuildGo(ctx context.Context, o GoBuild) ([]Receipt, error) {
	if o.Dir == "" {
		return nil, ErrNoToolsDir
	}
	if o.Run == nil {
		o.Run = runner.Run
	}
	if o.Out == nil {
		o.Out = io.Discard
	}
	receipts := make([]Receipt, 0, len(o.Tools))
	for _, rt := range o.Tools {
		rec, err := buildOne(ctx, o, rt)
		if err != nil {
			return receipts, err
		}
		receipts = append(receipts, rec)
	}
	return receipts, nil
}

// buildOne builds one tool unless the tool directory already holds it at
// the pin with its checksum. No GOOS or GOARCH is set, because sqlc's
// PostgreSQL parser builds with cgo for the host.
func buildOne(ctx context.Context, o GoBuild, rt runner.Tool) (Receipt, error) {
	t := GoTool(rt)
	resolver := Resolver{Dir: o.Dir, GOOS: o.GOOS, GOARCH: o.GOARCH, Env: o.Env, Run: o.Run}
	if rec, ok := resolver.verified(t); ok {
		return rec, say(o.Out, "%s %s já estava compilado e verificado em %s\n", t.Name, t.Version, rec.Path)
	}
	path := BinPath(o.Dir, t, o.GOOS, o.GOARCH)
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return Receipt{}, fmt.Errorf("%w: %s %s: tool directory: %w", ErrInstall, t.Name, t.Version, err)
	}
	args := []string{"build", "-modfile=tools/go.mod", "-trimpath", "-o", path, rt.Module}
	out, err := o.Run(ctx, runner.Command{Name: "go", Args: args, Dir: o.Root, Env: o.Env})
	if err != nil {
		return Receipt{}, fmt.Errorf("%w: %s %s: %w", ErrInstall, t.Name, t.Version, err)
	}
	if out.Code != 0 {
		return Receipt{}, fmt.Errorf("%w: %s %s: %s", ErrInstall, t.Name, t.Version, firstStderrLine(out))
	}
	if err := os.Chmod(path, execMode); err != nil {
		return Receipt{}, fmt.Errorf("%w: %s %s: %w", ErrInstall, t.Name, t.Version, err)
	}
	return record(InstallOptions{Dir: o.Dir, Out: o.Out}, t, path)
}

// firstStderrLine is the cause a failed build quotes: the go command says
// why on stderr, and its first non-empty line names the missing module or
// the compile error.
func firstStderrLine(out runner.Output) string {
	for line := range strings.Lines(string(out.Stderr)) {
		if text := strings.TrimSpace(line); text != "" {
			return text
		}
	}
	return fmt.Sprintf("go build exited with code %d", out.Code)
}

package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/runner"
)

func (b binding) goToolchainDoctor(ctx context.Context, root string) []doctor.Line {
	if _, err := os.Stat(filepath.Join(root, "go.mod")); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return doctor.AssessGo(b.goFacts(ctx, root))
}

// The probe must use the local toolchain even if the module requests a newer one.
func (b binding) goFacts(ctx context.Context, root string) doctor.GoFacts {
	facts := doctor.GoFacts{}
	out, err := runner.Run(ctx, runner.Command{
		Name: "go", Args: []string{"env", "GOVERSION"}, Dir: root,
		Env: append(runner.UserEnv(b.environ()), "GOTOOLCHAIN=local"),
	})
	if err != nil {
		facts.Error = fmt.Sprintf("go env GOVERSION: %v", err)
		return facts
	}
	if out.Code != 0 {
		facts.Error = fmt.Sprintf("go env GOVERSION: exit %d: %s", out.Code, strings.TrimSpace(string(out.Stderr)))
		return facts
	}
	facts.Local = strings.TrimSpace(string(out.Stdout))
	module, err := bootstrap.ReadModule(ctx, root, b.environ())
	if err != nil {
		facts.Error = "go mod edit -json: " + err.Error()
		return facts
	}
	facts.Line = module.GoVersion
	// Optional tools metadata must not hide the main module's diagnostic.
	toolsRoot := filepath.Join(root, "tools")
	if _, err = os.Stat(filepath.Join(toolsRoot, "go.mod")); err != nil {
		return facts
	}
	tools, err := bootstrap.ReadModule(ctx, toolsRoot, b.environ())
	if err != nil {
		return facts
	}
	facts.Tools = tools.GoVersion
	return facts
}

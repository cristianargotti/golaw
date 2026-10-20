package bootstrap

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/runner"
)

// ModuleInfo preserves the adopter's Go directive and direct dependencies.
type ModuleInfo struct {
	Path, GoVersion string
	Direct          []assets.Requirement
}

type moduleDocument struct {
	Module struct {
		Path string `json:"Path"`
	} `json:"Module"`
	Go      string              `json:"Go"`
	Require []moduleRequirement `json:"Require"`
}

type moduleRequirement struct {
	Path     string `json:"Path"`
	Version  string `json:"Version"`
	Indirect bool   `json:"Indirect"`
}

// ReadModule reads metadata without selecting a newer toolchain or editing go.mod.
func ReadModule(ctx context.Context, root string, env []string) (ModuleInfo, error) {
	out, err := runner.Run(ctx, runner.Command{
		Name: "go", Args: []string{"mod", "edit", "-json"}, Dir: root,
		Env: append(runner.UserEnv(env), "GOTOOLCHAIN=local"),
	})
	if err != nil {
		return ModuleInfo{}, fmt.Errorf("%w: %w", ErrModule, err)
	}
	if out.Code != 0 {
		return ModuleInfo{}, fmt.Errorf("%w: go mod edit exited %d: %s", ErrModule, out.Code, strings.TrimSpace(string(out.Stderr)))
	}
	var document moduleDocument
	if err := json.Unmarshal(out.Stdout, &document); err != nil {
		return ModuleInfo{}, fmt.Errorf("%w: decode go mod edit output: %w", ErrModule, err)
	}
	info := ModuleInfo{Path: document.Module.Path, GoVersion: document.Go}
	for _, requirement := range document.Require {
		if !requirement.Indirect {
			info.Direct = append(info.Direct, assets.Requirement{Path: requirement.Path, Version: requirement.Version})
		}
	}
	slices.SortFunc(info.Direct, func(a, b assets.Requirement) int { return cmp.Compare(a.Path, b.Path) })
	return info, nil
}

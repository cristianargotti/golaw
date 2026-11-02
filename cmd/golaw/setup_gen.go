package main

import (
	"context"
	"fmt"
	"io"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
)

// genFlag builds the pinned generators and golangci-lint.
const genFlag = "gen"

// genCapabilities names the capability that asks for each generator
// (M7.11.1), so a repository without a database never builds sqlc.
func genCapabilities() map[string]string {
	return map[string]string{"sqlc": "db", "oapi-codegen": "api", "goverter": "hexagonal"}
}

// setupGenCommand builds the tools the post hook may run from the
// repository's tools module. A person runs it; it is optional, and the
// pipeline runs the generators through go tool whatever this machine holds.
func (a *app) setupGenCommand(ctx context.Context, out io.Writer, args []string, surface, uninstall bool) error {
	switch {
	case surface:
		return setupFailed(fmt.Errorf("%w: --gen and --surface run apart: run golaw setup --surface, then golaw setup --gen", model.ErrUsage))
	case uninstall:
		return setupFailed(fmt.Errorf("%w: --gen and --uninstall do not combine: the built tools stay in the tool directory", model.ErrUsage))
	}
	return setupFailed(a.bind().setupGen(ctx, out, rootArg(args), a.now(), runner.Run))
}

// genTools answers golangci-lint, which the hook's format stage runs on
// every Go repository, then each generator whose capability the policy
// holds, in the name order of runner.Generators.
func genTools(pol *model.EffectivePolicy) []runner.Tool {
	var tools []runner.Tool
	if lint, ok := runner.LookupTool(linterTool); ok {
		tools = append(tools, lint)
	}
	capabilities := genCapabilities()
	for _, gen := range runner.Generators() {
		if pol.Has(capabilities[gen.Name]) {
			tools = append(tools, gen)
		}
	}
	return tools
}

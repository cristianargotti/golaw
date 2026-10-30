package lint

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// spec is the spec of the outcome of one tool, or of the inventory.
func spec(name string) model.Spec {
	return model.Spec{ID: "lint." + name, Rule: "rules/M6.9@1", Scope: model.ScopeTree, Owner: model.OwnerCore}
}

func notApplicable(reason string) model.Result {
	return model.Result{Status: model.NotApplicable(reason)}
}

func unmeasured(reason string) model.Result {
	return model.Result{Status: model.Unmeasured(reason)}
}

// runDriver runs one driver over the files of its languages in the scope.
// Every step that leaves the tool unable to judge answers why, so the
// person sees the one fix.
func runDriver(ctx context.Context, o Options, set string, inv surface.Inventory, d Driver, languages []surface.Language) model.Result {
	if set == model.SurfaceParity && !parityTool(d.Tool) {
		return notApplicable("no rule of the parity set for " + d.Tool)
	}
	paths, reason := files(inv, languages, o.Scope)
	if reason != "" {
		return notApplicable(reason)
	}
	in := Input{Root: o.Root, Files: paths, Set: set, Globals: surfacePolicy(o.Policy).Globals}
	if d.Config != nil {
		in.ConfigDir = toolchain.ConfigDir(o.Resolver.Dir, o.Kit, set)
	}
	args := d.Args(in)
	if args == nil {
		return notApplicable(d.Tool + " runs under strict only")
	}
	tool, ok := toolchain.Lookup(d.Tool)
	if !ok {
		return unmeasured(d.Tool + " is not pinned in the toolchain table")
	}
	resolved := o.Resolver.Full(ctx, tool)
	if resolved.State == toolchain.StateRuntime {
		// The runtime detail names Node or Python alone; without the tool
		// the person cannot tell which check it left unmeasured.
		return unmeasured(fmt.Sprintf("%s %s: %s", tool.Name, tool.Version, resolved.Detail))
	}
	if !resolved.Usable() {
		return unmeasured(resolved.Detail)
	}
	if d.Config != nil {
		if _, err := toolchain.Materialize(o.Resolver.Dir, o.Kit, set, d.Config); err != nil {
			return unmeasured(fmt.Sprintf("%s: the kit configuration: %v", d.Tool, err))
		}
	}
	out, failure := execute(ctx, o, d.Tool, resolved.Path, args)
	if failure != "" {
		return unmeasured(failure)
	}
	return parsed(d, out, o.Root, len(paths))
}

// parityTool reports whether the rule map holds a row of the tool at
// parity: under parity a tool without one has nothing to report.
func parityTool(tool string) bool {
	return slices.ContainsFunc(Rows(), func(r Row) bool { return r.Tool == tool && r.Set == model.SurfaceParity })
}

// files answers the source files of the languages inside the analyzer
// scope, in path order, or why there is none. One command line carries
// them all: a typed rule needs the whole program in one run.
func files(inv surface.Inventory, languages []surface.Language, scope surface.Scope) ([]string, string) {
	var present, inScope []string
	for _, f := range inv.Files {
		if !slices.Contains(languages, f.Language) || f.Class != surface.ClassSource {
			continue
		}
		present = append(present, f.Path)
		if scope.Role(f.Path) != surface.RoleOutside {
			inScope = append(inScope, f.Path)
		}
	}
	names := make([]string, 0, len(languages))
	for _, l := range languages {
		names = append(names, string(l))
	}
	list := strings.Join(names, " or ")
	switch {
	case len(present) == 0:
		return nil, fmt.Sprintf("no %s file", list)
	case len(inScope) == 0:
		return nil, fmt.Sprintf("no %s file in the analyzer scope", list)
	}
	return inScope, ""
}

// execute runs the tool in the repository root with the reduced
// environment and answers its output, or why it judged nothing.
func execute(ctx context.Context, o Options, tool, path string, args []string) (runner.Output, string) {
	got, err := toolchain.Exec(ctx, toolchain.Request{
		Path: path, Args: args, Dir: o.Root, Env: toolchain.ToolEnv(o.Resolver.Env), Timeout: ToolTimeout,
	})
	switch {
	case err != nil:
		return runner.Output{}, fmt.Sprintf("%s: %v", tool, err)
	case got.Killed:
		return runner.Output{}, tool + " passed the 10m budget of the lint target"
	}
	return got.Output, ""
}

// parsed turns the output into the result: a file the tool could not parse
// keeps the other files' findings and leaves the tool Unmeasured.
func parsed(d Driver, out runner.Output, root string, count int) model.Result {
	issues, err := Parse(d.Format, d.Tool, out, root)
	if err != nil && !errors.Is(err, ErrUnparsed) {
		return unmeasured(fmt.Sprintf("%s: %v", d.Tool, err))
	}
	result := model.Result{Status: model.Measured(), Scope: fmt.Sprintf("%d files", count)}
	for _, issue := range issues {
		result.Findings = append(result.Findings, ToFinding(issue))
	}
	if err != nil {
		result.Status = model.Unmeasured(err.Error())
	}
	return result
}

package main

import (
	"context"
	"slices"
	"time"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// surfaceDoctor says, for the languages the repository holds, which pinned
// tool can judge them on this machine and which runtime is missing. It
// never fails doctor: an unreadable surface is one unknown line.
func (b binding) surfaceDoctor(ctx context.Context, root string, now time.Time) []doctor.Line {
	snapshot, err := b.snapshot(ctx, root)
	if err != nil {
		return surfaceUnread(err)
	}
	inv, err := surface.Take(ctx, snapshot)
	if err != nil {
		return surfaceUnread(err)
	}
	facts := doctor.SurfaceFacts{Languages: languageNames(inv.Languages)}
	// The policy has its own doctor line, so a policy that cannot load
	// still lets the scope read the core defaults here.
	pol, err := b.load(ctx, root, now)
	if err != nil {
		core := model.CoreDefaults()
		pol = &core
	}
	scope, err := surface.ScopeOf(ctx, snapshot, pol)
	if err != nil {
		facts.Scope = []string{"unmeasured: " + err.Error()}
	} else {
		facts.Scope = scope.Lines()
	}
	if shape, outside := surface.OutsideShape(snapshot); outside {
		facts.Shape = shape
	}
	// A machine with no tools directory resolves every tool missing, which
	// names the fix instead of failing doctor.
	dirs, err := b.dirs()
	if err != nil {
		dirs = policy.Dirs{}
	}
	facts.Tools, facts.Runtimes = collectSurfaceTools(ctx, b.lintResolver(dirs), inv.Languages)
	return doctor.AssessSurface(facts)
}

func surfaceUnread(err error) []doctor.Line {
	return []doctor.Line{{ID: "doctor.surface-languages", State: "unknown", Code: "surface-languages", Detail: "the surface could not be read: " + err.Error()}}
}

func languageNames(langs []surface.Language) []string {
	names := make([]string, len(langs))
	for i, l := range langs {
		names[i] = string(l)
	}
	return names
}

// collectSurfaceTools resolves each tool of the held languages in full and
// each runtime one of them needs, so a runtime no held language needs
// earns no line.
func collectSurfaceTools(ctx context.Context, resolver toolchain.Resolver, held []surface.Language) ([]doctor.SurfaceTool, []doctor.SurfaceTool) {
	pinned := toolchain.ForLanguages(held)
	tools := make([]doctor.SurfaceTool, 0, len(pinned))
	needed := map[string]bool{}
	for i := range pinned {
		tool := &pinned[i]
		resolved := resolver.Full(ctx, *tool)
		var langs []surface.Language
		for _, l := range tool.Languages {
			if slices.Contains(held, l) {
				langs = append(langs, l)
			}
		}
		tools = append(tools, doctor.SurfaceTool{
			Name: tool.Name, Version: tool.Version, Languages: languageNames(langs),
			State: string(resolved.State), Path: resolved.Path, Detail: resolved.Detail,
		})
		needed[tool.Runtime()] = true
	}
	return tools, collectRuntimes(ctx, resolver, needed)
}

// collectRuntimes probes the runtimes the held tools name, node then
// python, as the table orders them.
func collectRuntimes(ctx context.Context, resolver toolchain.Resolver, needed map[string]bool) []doctor.SurfaceTool {
	var runtimes []doctor.SurfaceTool
	for _, rt := range toolchain.Runtimes() {
		if !needed[rt.Name] {
			continue
		}
		status := resolver.Runtime(ctx, rt)
		state := "missing"
		if status.OK {
			state = "verified"
		}
		runtimes = append(runtimes, doctor.SurfaceTool{Name: rt.Name, Version: status.Version, State: state, Path: status.Path, Detail: status.Detail})
	}
	return runtimes
}

package main

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/thirdparty"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/source"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// thirdOutcome is what the pinned generators of a post event did: the
// causes of the ones the hook could not run, the ones only golaw gen runs,
// and whether one ran, which asks for a new snapshot of its output.
type thirdOutcome struct {
	causes, outside []string
	ran             bool
}

// managedHashes maps each managed configuration of the policy in force to
// the sha256 golaw.yml records for it (spec 5.3), the only bytes a pinned
// generator may run over in the hook.
func (h hookBinding) managedHashes(ctx context.Context, root string) (map[string]string, error) {
	l, err := h.loaded(ctx, root, h.now)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(l.File.Managed))
	for _, m := range l.File.Managed {
		out[m.Path] = m.SHA256
	}
	return out, nil
}

// thirdParty runs the receipted generators in order under the event budget
// of external tools (M7.9.1 step 2). A generator it cannot run is a notice
// naming golaw gen, never a denial, and none runs from the repository or
// through the go command.
func (h hookBinding) thirdParty(ctx context.Context, root string, managed map[string]string, snap model.Snapshot, generators []thirdparty.Generator) thirdOutcome {
	event, cancel := h.budget.context(ctx)
	defer cancel()
	env := toolchain.ToolEnv(h.environ())
	var out thirdOutcome
	for _, g := range generators {
		if !g.Hook {
			out.outside = append(out.outside, g.Name)
			continue
		}
		cause, ran := h.runOne(event, root, managed, snap, g, env)
		if cause != "" {
			out.causes = append(out.causes, cause)
		}
		out.ran = out.ran || ran
	}
	return out
}

// runOne runs one generator, or answers the cause that kept it from
// running.
func (h hookBinding) runOne(ctx context.Context, root string, managed map[string]string, snap model.Snapshot, g thirdparty.Generator, env []string) (string, bool) {
	if ctx.Err() != nil {
		return g.Name + " was not started: the hook's 3 s budget for external tools was spent", false
	}
	if data, err := snap.Read(g.Config); err != nil || managed[g.Config] != source.ContentHash(data) {
		return g.Config + " differs from the managed configuration golaw.yml records", false
	}
	path, missing := h.generatorPath(g.Name)
	if missing != "" {
		return missing, false
	}
	run, err := toolchain.Exec(ctx, toolchain.Request{Path: path, Args: g.Args, Dir: root, Env: env, Timeout: h.toolTimeout()})
	switch {
	case err != nil && ctx.Err() != nil:
		return g.Name + " passed the hook's 3 s budget for external tools", false
	case err != nil:
		return g.Name + " did not start: " + err.Error(), false
	case run.Killed:
		return g.Name + " passed the hook's 1.5 s limit", false
	case run.Output.Code != 0:
		return fmt.Sprintf("%s exited %d: %s", g.Name, run.Output.Code, firstLine(run.Output.Stderr)), false
	}
	return "", true
}

// generatorPath resolves the pinned binary from its receipt alone, so the
// hook starts nothing to find it (M6.5.2).
func (h hookBinding) generatorPath(name string) (string, string) {
	pins := runner.Generators()
	i := slices.IndexFunc(pins, func(t runner.Tool) bool { return t.Name == name })
	if i < 0 {
		return "", name + " is not a generator golaw pins: run golaw gen"
	}
	resolver := toolchain.Resolver{Dir: h.dirs.Tools, GOOS: h.goos, GOARCH: runtime.GOARCH}
	resolved := resolver.Hook(toolchain.GoTool(pins[i]))
	if !resolved.Usable() {
		return "", fmt.Sprintf("%s %s is not built on this machine: run golaw setup --gen", name, pins[i].Version)
	}
	return resolved.Path, ""
}

// firstLine is the first non-empty line of a tool's stderr, the line that
// names why it failed.
func firstLine(stderr []byte) string {
	for line := range strings.Lines(string(stderr)) {
		if text := strings.TrimSpace(line); text != "" {
			return text
		}
	}
	return "no output on stderr"
}

// regenThird runs the generators an event triggered before the kinds
// render, and answers the snapshot the kinds then read: a new one when a
// generator wrote, because the handler and SQL adapter kinds read its
// output (step 2 then step 1).
func (h hookBinding) regenThird(ctx context.Context, root string, snap model.Snapshot, third []thirdparty.Generator, o *regenOutcome) model.Snapshot {
	if len(third) == 0 {
		return snap
	}
	managed, err := h.managedHashes(ctx, root)
	if err != nil {
		o.causes = append(o.causes, err.Error())
		return snap
	}
	out := h.thirdParty(ctx, root, managed, snap, third)
	o.causes, o.outside = append(o.causes, out.causes...), append(o.outside, out.outside...)
	if !out.ran {
		return snap
	}
	// The cached listing predates the generator's new files, so the
	// listing is taken again; only an event that ran a generator pays it.
	fresh, err := h.binding.snapshot(ctx, root)
	if err != nil {
		o.causes = append(o.causes, err.Error())
		return snap
	}
	return fresh
}

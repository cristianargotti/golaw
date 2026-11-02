package main

import (
	"context"
	"time"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// generateDoctor says, for each tool golaw setup --gen builds under the
// policy's capabilities, whether the post hook can run it here. It never
// fails doctor: a missing binary is a line that names the fix.
func (b binding) generateDoctor(ctx context.Context, root string, now time.Time) []doctor.Line {
	// The policy has its own doctor line, so a policy that cannot load
	// still lets the core defaults name the linter here.
	pol, err := b.load(ctx, root, now)
	if err != nil {
		core := model.CoreDefaults()
		pol = &core
	}
	dirs, err := b.dirs()
	if err != nil {
		dirs = policy.Dirs{}
	}
	resolver := b.lintResolver(dirs)
	var facts doctor.GenerateFacts
	for _, rt := range genTools(pol) {
		r := resolver.Full(ctx, toolchain.GoTool(rt))
		facts.Tools = append(facts.Tools, doctor.GenerateTool{
			Name: rt.Name, Version: rt.Version, State: string(r.State), Path: r.Path, Detail: r.Detail,
		})
	}
	return doctor.AssessGenerate(facts)
}

package bootstrap

import (
	"slices"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
)

// HoldRecords keeps every record golaw.yml holds, for a policy init may
// not change: a template that would gain or change its record is proposed
// instead of written and named in Held, so the policy keeps its bytes and
// its owners can commit the file and the record by hand.
func (plan *TemplatePlan) HoldRecords() {
	var held []policy.ManagedFile
	for _, record := range plan.Managed {
		if !slices.Contains(plan.baselines, record) {
			held = append(held, record)
		}
	}
	changes := make([]install.Change, 0, len(plan.Changes))
	proposed := slices.Clone(plan.Proposed)
	for _, change := range plan.Changes {
		if slices.ContainsFunc(held, func(record policy.ManagedFile) bool { return record.Path == change.Path }) {
			proposed = append(proposed, change)
			continue
		}
		changes = append(changes, change)
	}
	plan.Changes, plan.Proposed, plan.Held = changes, proposed, held
	plan.Managed = slices.Clone(plan.baselines)
}

package main

import (
	"context"
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// updateCapabilities refuses --capabilities beside --update: the update
// re-renders the managed files at the policy's own capabilities and moves
// the pin, so a capability named there would be dropped without a word.
func updateCapabilities(opts initOptions) error {
	if len(opts.capabilities) == 0 {
		return nil
	}
	return fmt.Errorf("%w: --update keeps the capabilities of golaw.yml; run golaw init --capabilities first, commit golaw.yml, then golaw init --update", model.ErrUsage)
}

// grownAtHead reports whether init grew the capabilities of the policy HEAD
// holds. Such a policy is enrolled once it is committed, not before: while
// HEAD holds the policy the record enrolled, the commit that brings the new
// one passes the armed rungs and the agent hooks keep working, which a
// record ahead of HEAD would refuse as a mismatch.
// An earlier apply whose growth is not committed yet leaves the record at
// the policy HEAD holds, so a repeated apply waits for the commit too.
func (a *app) grownAtHead(ctx context.Context, root string, plan initPlan) (bool, error) {
	if plan.update {
		return false, nil
	}
	current := digestOf(plan.current)
	if current != "" && current != digestOf(plan.data) {
		committed, err := a.committed(ctx, root, current)
		if err != nil || committed {
			return committed, err
		}
	}
	held, found, err := a.capabilityBaseline(root)
	if err != nil {
		return false, err
	}
	if !found || len(held.Capabilities) >= len(plan.policy.Capabilities) {
		return false, nil
	}
	return a.committed(ctx, root, policy.Digest(held))
}

// capabilityBaseline keeps repeated applies relative to the enrolled policy,
// so an earlier uncommitted growth never advances the record ahead of HEAD.
func (a *app) capabilityBaseline(root string) (policy.File, bool, error) {
	dirs, err := a.bind().dirs()
	if err != nil {
		return policy.File{}, false, initOperation("read capability enrollment", err)
	}
	record, found, err := policy.ReadRecord(dirs.State, root)
	if err != nil {
		return policy.File{}, false, initOperation("read capability enrollment", err)
	}
	if !found || record.Bootstrap == "" {
		return policy.File{}, false, nil
	}
	config, err := policy.Decode([]byte(record.Bootstrap))
	return config, true, initOperation("decode capability baseline", err)
}

// enrollAfterCommitLine asks for the commit of the grown policy and the run
// of init that enrolls it, in the language of the policy.
func enrollAfterCommitLine(language string) string {
	if language == model.DefaultLanguage {
		return "Faça o commit de golaw.yml e rode golaw init --apply outra vez para registrar a política com as capacidades novas."
	}
	return "Commit golaw.yml, then run golaw init --apply again to enroll the policy with its new capabilities."
}

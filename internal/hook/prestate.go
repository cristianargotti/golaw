package hook

import (
	"context"
	"fmt"
	"maps"

	"github.com/cristianargotti/golaw/internal/model"
)

func (run execution) saveBefore(ctx context.Context, data []byte, dependencies Dependencies) {
	optionalAction(ctx, func(bounded context.Context) {
		run.persistBefore(bounded, data, dependencies)
	})
}

func (run execution) persistBefore(ctx context.Context, data []byte, dependencies Dependencies) {
	key, err := StateKey(run.repository.Identity, run.request.Event)
	if err != nil {
		return
	}
	before, reason := run.beforeCapture(ctx, dependencies)
	inputHash := Hash([]byte(joinParts([]string{
		run.policy.Digest, Hash(data), MapHash(before), reason,
	})))
	if err := dependencies.State.Save(ctx, key, inputHash, before, reason); err != nil {
		if retryErr := dependencies.State.Save(ctx, key, inputHash, nil, err.Error()); retryErr != nil {
			return
		}
	}
}

func (run execution) beforeCapture(ctx context.Context, dependencies Dependencies) (map[string]string, string) {
	if run.stateReason != "" {
		return nil, run.stateReason
	}
	before, err := run.captureIdentities(ctx, dependencies)
	if err != nil {
		return nil, err.Error()
	}
	return before, ""
}

func (run execution) checkWriteFresh(ctx context.Context, dependencies Dependencies) error {
	paths := candidatePaths(run.request.Event.Candidates)
	before, err := Identities(ctx, run.base, paths)
	if err != nil {
		return err
	}
	return run.checkFresh(ctx, paths, before, dependencies)
}

func (run execution) checkFresh(ctx context.Context, paths []string, before map[string]string, dependencies Dependencies) error {
	current, err := dependencies.Snapshot(ctx, run.repository)
	if err != nil {
		return fmt.Errorf("%w: refresh candidate base: %w", model.ErrCannotJudge, err)
	}
	identities, err := Identities(ctx, current, paths)
	if err != nil {
		return err
	}
	if !maps.Equal(before, identities) {
		return fmt.Errorf("%w: candidate base changed before the answer", model.ErrCannotJudge)
	}
	return nil
}

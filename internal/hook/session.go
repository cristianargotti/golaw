package hook

import "context"

// session answers a session start of an enrolled repository: the map its
// binding gives as context, or a plain allow without one, because a
// session start never denies (M7.8).
func (run execution) session(ctx context.Context, dependencies Dependencies) Result {
	if dependencies.Session == nil {
		return Result{Outcome: Allow}
	}
	return Result{Outcome: Allow, Notice: dependencies.Session(ctx, run.repository, run.policy)}
}

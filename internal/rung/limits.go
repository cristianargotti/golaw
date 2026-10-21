package rung

import (
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// limited counts every check a stated limit of its declared subset left
// unmeasured as measured, and answers one line of text per such check, the
// check id then its reason. A rung refuses only what it recognizes with
// certainty, so the construct is named and left to the pipeline, whose scan
// still counts the check unmeasured (section 5.2).
func limited(run *engine.Run) []string {
	var limits []string
	for i := range run.Outcomes {
		res := &run.Outcomes[i].Result
		if model.IsOutsideSubset(res.Status) {
			limits = append(limits, run.Outcomes[i].Spec.ID+" left to the pipeline: "+res.Status.Reason)
			res.Status = model.Measured()
		}
	}
	return limits
}

// limitLines names each stated limit of a run in its own line, prefixed
// with the rung, because git prints the output of a hook with no frame.
func limitLines(name string, limits []string) []string {
	lines := make([]string, 0, len(limits))
	for _, limit := range limits {
		lines = append(lines, prefix(name)+limit)
	}
	return lines
}

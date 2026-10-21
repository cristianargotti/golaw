package hook

import (
	"context"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// judged is the judgment of the rules, with the files the text of a shell
// pre event writes judged as a write first: a write the rules refuse
// answers the event, an allowed one lets the command be judged, and its
// notice stays beside the notice of the command.
func (run execution) judged(ctx context.Context, data []byte, dependencies Dependencies) Result {
	writes := run.request.Writes
	if writes == nil || run.request.Event.Shape != model.ShapeShell {
		return run.evaluate(ctx, data, dependencies)
	}
	write := run
	write.request = Request{Event: run.request.Event, Content: writes.Content}
	write.request.Event.Shape, write.request.Event.Shell = model.ShapeWrite, nil
	write.request.Event.Candidates = writes.Candidates
	written := write.evaluate(ctx, nil, dependencies)
	if written.Outcome != Allow {
		return written
	}
	return joined(written, run.evaluate(ctx, data, dependencies))
}

// joined keeps the notice of an allowed write beside the answer of its
// command when the command is allowed too.
func joined(write, command Result) Result {
	if command.Outcome != Allow || write.Notice == "" {
		return command
	}
	command.Notice = strings.TrimSpace(write.Notice + " " + command.Notice)
	command.Human = strings.TrimSpace(write.Human + " " + command.Human)
	return command
}

// unapplied names each candidate the decoder could not reconstruct, whose
// added lines alone were judged, so the agent learns its size and its
// structure were not.
func unapplied(candidates []model.Candidate) []Gap {
	var gaps []Gap
	for _, c := range candidates {
		if !c.Reconstructed {
			gaps = append(gaps, Gap{Path: c.Path, Cause: causeUnapplied})
		}
	}
	return gaps
}

package hook

import (
	"context"
	"fmt"
	"io"

	"github.com/cristianargotti/golaw/internal/model"
)

// evaluateOversized answers an event whose input is over the bound as any
// event, from the parse the adapter reads in the whole input in bounded
// memory: the same validation, an unenrolled location allowed, and the
// request the adapter answers without the bytes it could not hold, so a
// write named by its path is judged by the name rules over a snapshot that
// reads that path as over the bound, and a post event is judged from its
// header with its detection.
func evaluateOversized(ctx context.Context, agent string, input io.Reader, adapter Adapter, dependencies Dependencies) completed {
	return evaluateParsed(ctx, agent, func() (Parsed, error) { return oversizedParse(adapter, input) }, nil, dependencies)
}

// oversizedParse parses an input over the bound with the adapter, then reads
// the rest and drops it, so the host that writes it never meets a closed
// pipe; without a reader the input is the bound.
func oversizedParse(adapter Adapter, input io.Reader) (Parsed, error) {
	parsed, err := Parsed{}, inputBound()
	if adapter.Oversized != nil {
		parsed, err = adapter.Oversized(input)
	}
	if _, drained := io.Copy(io.Discard, input); drained != nil && err == nil {
		err = fmt.Errorf("%w: read input: %w", model.ErrCannotJudge, drained)
	}
	return parsed, err
}

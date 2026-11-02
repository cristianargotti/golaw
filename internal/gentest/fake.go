package gentest

import (
	"fmt"
	"slices"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
)

// FakeContracts implements genkind.Contracts from fixed answers, so a kind
// test proves its rendering over a contract without the parser underneath.
type FakeContracts struct {
	PortAnswers      map[string]genkind.Port // key "<path>#<symbol>"
	OperationAnswers []genkind.Operation
	QueryAnswers     []genkind.Query
}

// Port answers the programmed port; an unprogrammed one is the error the
// real reader gives a port it cannot judge, so a kind's refusal path runs.
func (f FakeContracts) Port(path, symbol string) (genkind.Port, error) {
	port, ok := f.PortAnswers[path+"#"+symbol]
	if !ok {
		return genkind.Port{}, fmt.Errorf("%w: no fake port %s in %s", genkind.ErrCannotJudge, symbol, path)
	}
	return port, nil
}

// Operations answers a clone, so a kind that sorts its operations never
// reorders what another call reads.
func (f FakeContracts) Operations(_, _ string) ([]genkind.Operation, error) {
	return slices.Clone(f.OperationAnswers), nil
}

// Queries answers a clone for the same reason as Operations.
func (f FakeContracts) Queries(_ string) ([]genkind.Query, error) {
	return slices.Clone(f.QueryAnswers), nil
}

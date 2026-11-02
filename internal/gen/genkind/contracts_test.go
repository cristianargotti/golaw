package genkind_test

import (
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gentest"
)

// The fake every kind test passes must stay a genkind.Contracts, so a change
// to the interface fails here before it fails in each kind's tests.
var _ genkind.Contracts = gentest.FakeContracts{}

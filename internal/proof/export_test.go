package proof

import "github.com/cristianargotti/golaw/internal/engine"

// Matrix is the condition pass for the tests alone: no production code runs
// it, and the test support that imports this package from production files
// would make an exported pass a symbol without a production consumer.
func Matrix(reg *engine.Registry) []Failure { return matrix(reg) }

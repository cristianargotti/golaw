package surface

import "github.com/cristianargotti/golaw/internal/model"

// Checks builds the checks of the family in report order.
func Checks() []model.Check {
	return []model.Check{CodeEval(), HardcodedCredential(), FileLength(), PythonFunctionLength()}
}

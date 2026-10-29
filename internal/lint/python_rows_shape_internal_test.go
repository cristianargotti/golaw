package lint

import "testing"

func TestPythonRowsHoldTheShapeEdits(t *testing.T) {
	t.Parallel()
	pythonExpectRows(t, []Row{
		pythonRuffRow("python:S1066", "SIM102", "close"),
		pythonRuffRow("python:S1135", "FIX002", "close"),
		pythonRuffRow("python:S1172", "ARG001", "close"),
		pythonRuffRow("python:S117", "N802", "close"),
		pythonRuffRow("python:S117", "N806", "close"),
		pythonRuffRow("python:S125", "ERA001", "close"),
		pythonRuffRow("python:S1481", "F841", "close"),
		pythonRuffRow("python:S5806", "A001", "close"),
		pythonRuffRow("python:S6903", "DTZ003", "close"),
	})
	if got := len(pythonRows()); got != 31 {
		t.Errorf("the python rule map holds %d rows, want the 31 of M6.4.3", got)
	}
}

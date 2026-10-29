package lint

import "testing"

func TestPythonRowsHoldTheSafetyEdits(t *testing.T) {
	t.Parallel()
	pythonExpectRows(t, []Row{
		pythonRuffRow("python:S2077", "S608", "close"),
		pythonRuffRow("python:S2245", "S311", "close"),
		pythonRuffRow("python:S4721", "S602", "close"),
		pythonRuffRow("python:S4721", "S605", "close"),
		pythonRuffRow("python:S4790", "S324", "close"),
		pythonRuffRow("python:S4830", "S501", "close"),
		pythonRuffRow("python:S5042", "S202", "close"),
		pythonRuffRow("python:S5247", "S701", "close"),
		pythonRuffRow("python:S5443", "S108", "close"),
		pythonRuffRow("python:S7494", "C402", "close"),
		pythonRuffRow("python:S7494", "C408", "close"),
	})
}

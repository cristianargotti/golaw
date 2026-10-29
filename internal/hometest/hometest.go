// Package hometest gives the tests of every package that writes outside a
// checkout an isolated home: the home, state, config and data directories
// of each OS under the temporary directory of the test, so no test reads or
// writes the developer's agent configuration or tool directory. Only tests
// import it.
package hometest

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// dirMode keeps each directory of the home private to the test user.
const dirMode = 0o700

// Home is one isolated home of a test.
type Home struct {
	Dir    string // HOME and USERPROFILE
	State  string // XDG_STATE_HOME and LOCALAPPDATA
	Config string // XDG_CONFIG_HOME and APPDATA
	Data   string // XDG_DATA_HOME
}

// New creates the four directories under tb.TempDir.
func New(tb testing.TB) Home {
	tb.Helper()
	root := tb.TempDir()
	h := Home{
		Dir:    filepath.Join(root, "home"),
		State:  filepath.Join(root, "state"),
		Config: filepath.Join(root, "config"),
		Data:   filepath.Join(root, "data"),
	}
	for _, dir := range []string{h.Dir, h.State, h.Config, h.Data} {
		if err := os.MkdirAll(dir, dirMode); err != nil {
			tb.Fatal(err)
		}
	}
	return h
}

// Getenv answers a variable of the home, empty for any other name, in the
// shape of the env functions policy.DefaultDirs takes.
func (h Home) Getenv(name string) string { return h.variables()[name] }

// Environ answers the variables as KEY=value lines, sorted, for a process
// a test starts.
func (h Home) Environ() []string {
	out := make([]string, 0, len(h.variables()))
	for name, value := range h.variables() {
		out = append(out, name+"="+value)
	}
	slices.Sort(out)
	return out
}

// Tools is the tools directory policy.ToolsDir answers for this home on
// every OS, because XDG_DATA_HOME is set.
func (h Home) Tools() string { return filepath.Join(h.Data, "golaw", "tools") }

// variables maps every variable the home answers to its value. PATH is
// not among them: a test that starts a process names the PATH it means,
// the directory of its fake tools first.
func (h Home) variables() map[string]string {
	return map[string]string{
		"HOME": h.Dir, "USERPROFILE": h.Dir,
		"XDG_STATE_HOME": h.State, "LOCALAPPDATA": h.State,
		"XDG_CONFIG_HOME": h.Config, "APPDATA": h.Config,
		"XDG_DATA_HOME": h.Data,
	}
}

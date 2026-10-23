package profile

import "testing"

// TestCauseLineNamesTheFatalLineOfGit reads the line that names the cause
// from what git writes on its error stream: the first fatal line ahead of
// a hint of several lines, else the last line, else a line saying git
// wrote nothing.
func TestCauseLineNamesTheFatalLineOfGit(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, stderr, want string }{
		{"a hint after the cause", "fatal: 'C:/w/absent.git' does not appear to be a git repository\nfatal: Could not read from remote repository.\n\nPlease make sure you have the correct access rights\nand the repository exists.\n", "fatal: 'C:/w/absent.git' does not appear to be a git repository"},
		{"a warning before the cause", "warning: Could not find remote branch v9.9.9 to clone.\nfatal: Remote branch v9.9.9 not found in upstream origin\n", "fatal: Remote branch v9.9.9 not found in upstream origin"},
		{"no fatal line", "error: something failed\r\n  the last line\r\n", "the last line"},
		{"nothing", " \n", "git failed without a message"},
	} {
		if got := causeLine([]byte(tc.stderr)); got != tc.want {
			t.Errorf("%s: causeLine = %q, want %q", tc.name, got, tc.want)
		}
	}
}

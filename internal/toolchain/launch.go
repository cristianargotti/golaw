package toolchain

import (
	"bufio"
	"io"
	"os"
	"slices"
	"strings"
)

// maxShebang bounds what launch reads of a file to find its #! line.
const maxShebang = 512

// launch answers the program and arguments that start path. A script whose
// #! line names an interpreter path holding a space, as a virtual
// environment under a home with a space writes for a plain script such as
// checkov's, cannot start through the kernel, which splits that line at the
// space; its interpreter then starts with the script as the first argument.
// Windows reads no #! line at all, so there every such script starts
// through its interpreter. Every other file starts as it is.
func launch(path string, args []string, goos string) (string, []string) {
	interpreter, found := scriptInterpreter(path)
	if !found || goos != goosWindows && !strings.Contains(interpreter, " ") {
		return path, args
	}
	return interpreter, slices.Concat([]string{path}, args)
}

// scriptInterpreter reads the #! line of path and answers its interpreter,
// unquoted, when that path names a file.
func scriptInterpreter(path string) (string, bool) {
	f, err := os.Open(path) //nolint:gosec // path is the tool the resolver chose to run
	if err != nil {
		return "", false
	}
	defer func() { _ = f.Close() }()
	line, err := bufio.NewReader(io.LimitReader(f, maxShebang)).ReadString('\n')
	rest, found := strings.CutPrefix(line, "#!")
	interpreter := strings.Trim(strings.TrimSpace(rest), `"`)
	if err != nil || !found || interpreter == "" {
		return "", false
	}
	if info, statErr := os.Stat(interpreter); statErr != nil || info.IsDir() {
		return "", false
	}
	return interpreter, true
}

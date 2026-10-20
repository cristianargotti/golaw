// Package runnertest plays the pinned tools in the tests of the packages
// that run them: a fake tool is a copy of the running test binary that,
// started under the tool's name, answers what its test chose and records
// every call, so a test proves what a rung ran without the real tool, the
// network or a second build.
package runnertest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// BrokenCode is the exit status of a fake whose behavior or call record
// cannot be read or written, so the test that started it fails loudly.
const BrokenCode = 125

// The files a fake keeps beside itself.
const (
	exeSuffix      = ".exe"
	behaviorSuffix = ".behavior.json"
	callsSuffix    = ".calls.jsonl"
	executableMode = 0o700
	fileMode       = 0o600
)

// Behavior is what a fake tool answers.
type Behavior struct {
	Probe   []string `json:"probe"`   // the arguments that print the version: runner.Tool.Probe
	Version string   `json:"version"` // printed with a newline for the probe
	Stdout  string   `json:"stdout"`  // printed for any other arguments
	Stderr  string   `json:"stderr"`
	Code    int      `json:"code"` // the exit status for any other arguments
}

// Main is the body of the TestMain of a package whose tests install fake
// tools: a process started as a fake tool answers as one and returns its
// code without running a test; any other process runs the tests and
// returns m.Run(). The package's TestMain passes the code to os.Exit.
func Main(m *testing.M) int {
	if exe, err := os.Executable(); err == nil { // fallback: a binary that cannot name itself is no fake and runs its tests
		if code, fake := play(exe, os.Args[1:], os.Stdout, os.Stderr); fake {
			return code
		}
	}
	return m.Run()
}

// Install puts in dir a fake tool named name (name.exe on Windows), a copy
// of the running test binary that answers b, and answers its path; a
// second Install of the same name in the same dir replaces what it answers
// and starts a new call record.
func Install(tb testing.TB, dir, name string, b Behavior) string {
	tb.Helper()
	path := filepath.Join(dir, name)
	if runtime.GOOS == "windows" {
		path += exeSuffix
	}
	base := strings.TrimSuffix(path, exeSuffix)
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		copyExecutable(tb, path)
	}
	data, err := json.Marshal(b)
	if err != nil {
		tb.Fatalf("encode the behavior of %s: %v", name, err)
	}
	if err := os.WriteFile(base+behaviorSuffix, data, fileMode); err != nil {
		tb.Fatalf("write the behavior of %s: %v", name, err)
	}
	if err := os.Remove(base + callsSuffix); err != nil && !errors.Is(err, fs.ErrNotExist) {
		tb.Fatalf("reset the calls of %s: %v", name, err)
	}
	return path
}

// Calls answers the argument lists the fake tool name in dir was started
// with, in order, none for a tool never started.
func Calls(tb testing.TB, dir, name string) [][]string {
	tb.Helper()
	return readCalls(tb, filepath.Join(dir, name))
}

// play answers as the fake tool at exe when a behavior sits beside it, and
// reports false for any other binary.
func play(exe string, args []string, stdout, stderr io.Writer) (int, bool) {
	base := strings.TrimSuffix(exe, exeSuffix)
	data, err := os.ReadFile(base + behaviorSuffix) //nolint:gosec // the fake reads the behavior its test wrote beside it
	if errors.Is(err, fs.ErrNotExist) {
		return 0, false
	}
	var b Behavior
	if err == nil {
		err = json.Unmarshal(data, &b)
	}
	if err != nil {
		complain(stderr, "read its behavior", err)
		return BrokenCode, true
	}
	if err := record(base, args); err != nil {
		complain(stderr, "record its call", err)
		return BrokenCode, true
	}
	if slices.Equal(args, b.Probe) {
		return answer(stdout, nil, b.Version+"\n", "", 0), true
	}
	return answer(stdout, stderr, b.Stdout, b.Stderr, b.Code), true
}

// answer prints what a fake answers and its code, BrokenCode when a stream
// refuses the bytes.
func answer(stdout, stderr io.Writer, out, errText string, code int) int {
	if _, err := io.WriteString(stdout, out); err != nil {
		return BrokenCode
	}
	if stderr != nil {
		if _, err := io.WriteString(stderr, errText); err != nil {
			return BrokenCode
		}
	}
	return code
}

// complain tells the test why its fake cannot answer; a stream that
// refuses the line leaves BrokenCode as the only signal.
func complain(stderr io.Writer, what string, err error) {
	if _, writeErr := fmt.Fprintf(stderr, "fake tool: %s: %v\n", what, err); writeErr != nil {
		return
	}
}

// record appends one JSON line per call, because every start of a fake is
// a process of its own and only an append keeps each call, in order, for
// the test that reads them.
func record(base string, args []string) error {
	line, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("encode the call: %w", err)
	}
	file, err := os.OpenFile(base+callsSuffix, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fileMode) //nolint:gosec // the record sits beside the fake its test installed
	if err != nil {
		return fmt.Errorf("open the record: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write the record: %w", err)
	}
	return nil
}

// readCalls decodes the record as a stream of JSON values rather than as
// lines, so a call of any length, a tool handed many staged paths among
// them, stays one call, and a record it cannot decode fails the test
// instead of hiding the calls after it.
func readCalls(tb testing.TB, base string) [][]string {
	tb.Helper()
	data, err := os.ReadFile(base + callsSuffix) //nolint:gosec // the record of a fake the test installed
	if errors.Is(err, fs.ErrNotExist) {
		return [][]string{}
	}
	if err != nil {
		tb.Fatalf("read the calls of %s: %v", base, err)
	}
	var calls [][]string
	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		var args []string
		switch err := decoder.Decode(&args); {
		case errors.Is(err, io.EOF):
			return calls
		case err != nil:
			tb.Fatalf("decode a call of %s: %v", base, err)
			return calls
		}
		calls = append(calls, args)
	}
}

// copyExecutable copies the running test binary to path, executable.
func copyExecutable(tb testing.TB, path string) {
	tb.Helper()
	exe, err := os.Executable()
	if err != nil {
		tb.Fatalf("find the test binary: %v", err)
	}
	data, err := os.ReadFile(exe) //nolint:gosec // the running test binary
	if err != nil {
		tb.Fatalf("read the test binary: %v", err)
	}
	if err := os.WriteFile(path, data, executableMode); err != nil { //nolint:gosec // the fake lands in the directory its test chose
		tb.Fatalf("write the fake %s: %v", path, err)
	}
}

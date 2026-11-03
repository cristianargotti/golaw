package check

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// perTest bounds the messages kept of one failed test; the count line of
// the report already says how many tests failed.
const perTest = 3

// maxWidth bounds one failure line, so a dumped value stays one line.
const maxWidth = 240

// fileLinePattern matches the file:line: a test log line starts with.
var fileLinePattern = regexp.MustCompile(`^([\w.\-]+\.go):(\d+): `)

// testEvent is the part of a go test -json event the report reads.
type testEvent struct {
	Action  string `json:"Action"`
	Package string `json:"Package"`
	Test    string `json:"Test"`
	Output  string `json:"Output"`
}

// testRun gathers the events of one go test -json stream.
type testRun struct {
	outputs  map[string][]string // per package and test, the output lines
	failed   []string            // the keys of the failed tests, in order
	packages []string            // the packages that failed, in order
	build    []string            // the build errors, in order
}

// maxEvent bounds one line of the stream; an event is one output line of a
// test, so only a test that prints megabytes without a newline reaches it.
const maxEvent = 16 << 20

// firstEvent is the buffer a scan starts with, grown up to maxEvent.
const firstEvent = 64 << 10

// TestFailures reads a go test -json stream and answers, build errors
// first, one line per message of each failed leaf test with its file made
// relative to the module through dirOf, then the packages that failed with
// no failed test, such as a TestMain that exited or a timeout. It streams,
// so a suite of any size keeps the events at its end.
func TestFailures(stream io.Reader, dirOf func(pkg string) string) ([]string, error) {
	run := testRun{outputs: map[string][]string{}}
	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 0, firstEvent), maxEvent)
	for scanner.Scan() {
		run.read(scanner.Bytes())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read the go test events: %w", err)
	}
	out := slices.Clone(run.build)
	for _, key := range run.leaves() {
		pkg, test, _ := strings.Cut(key, "\x00")
		out = append(out, testLines(test, dirOf(pkg), run.outputs[key])...)
	}
	for _, pkg := range run.packages {
		if !slices.ContainsFunc(run.failed, func(key string) bool { return strings.HasPrefix(key, pkg+"\x00") }) {
			out = append(out, packageLines(pkg, run.outputs[pkg+"\x00"])...)
		}
	}
	return out, nil
}

func (r *testRun) read(line []byte) {
	if len(bytes.TrimSpace(line)) == 0 {
		return
	}
	var event testEvent
	if err := json.Unmarshal(line, &event); err != nil || event.Action == "" {
		r.addBuild(string(line)) // go prints a setup failure as plain text
		return
	}
	key := event.Package + "\x00" + event.Test
	switch event.Action {
	case "build-output":
		r.addBuild(event.Output)
	case "output":
		r.outputs[key] = append(r.outputs[key], event.Output)
	case "fail":
		if event.Test != "" {
			r.failed = append(r.failed, key)
		} else if event.Package != "" {
			r.packages = append(r.packages, event.Package)
		}
	}
}

// addBuild keeps a compiler line and drops the # package header.
func (r *testRun) addBuild(text string) {
	for _, line := range nonBlank(text) {
		if !strings.HasPrefix(line, "#") {
			r.build = append(r.build, clip(strings.TrimPrefix(line, "./")))
		}
	}
}

// leaves drops a parent whose subtest failed, because the subtest holds
// the message and the parent only repeats that a child failed.
func (r *testRun) leaves() []string {
	var out []string
	for _, key := range r.failed {
		parent := slices.ContainsFunc(r.failed, func(other string) bool { return strings.HasPrefix(other, key+"/") })
		if !parent {
			out = append(out, key)
		}
	}
	return out
}

// testLines answers the messages of one failed test, each led by the test
// name; a line that carries no file:line continues the message before it.
func testLines(test, dir string, output []string) []string {
	var messages []string
	for _, line := range detail(output) {
		match := fileLinePattern.FindStringSubmatch(line)
		switch {
		case match != nil:
			messages = append(messages, relative(dir, match[1])+line[len(match[1]):])
		case len(messages) > 0:
			messages[len(messages)-1] += " " + line
		default:
			messages = append(messages, line)
		}
	}
	if len(messages) == 0 {
		return []string{test + ": failed"}
	}
	out := make([]string, 0, perTest)
	for _, message := range messages[:min(len(messages), perTest)] {
		out = append(out, clip(test+": "+message))
	}
	return out
}

// packageLines answers what a package printed outside its tests.
func packageLines(pkg string, output []string) []string {
	lines := detail(output)
	if len(lines) == 0 {
		return []string{pkg + ": failed"}
	}
	out := make([]string, 0, perTest)
	for _, line := range lines[:min(len(lines), perTest)] {
		out = append(out, clip(pkg+": "+line))
	}
	return out
}

// detail keeps the lines a test or package logged and drops the frames go
// test prints around them.
func detail(output []string) []string {
	var out []string
	for _, line := range nonBlank(strings.Join(output, "")) {
		switch {
		case strings.HasPrefix(line, "=== "), strings.HasPrefix(line, "--- "),
			line == "FAIL", line == "PASS", strings.HasPrefix(line, "FAIL\t"),
			strings.HasPrefix(line, "ok "), strings.HasPrefix(line, "coverage: "):
		default:
			out = append(out, line)
		}
	}
	return out
}

// relative names a file of a test log from the module root.
func relative(dir, file string) string {
	if dir == "" || dir == "." {
		return file
	}
	return dir + "/" + file
}

func clip(line string) string {
	if len(line) <= maxWidth {
		return line
	}
	cut := maxWidth
	for cut > 0 && !utf8.RuneStart(line[cut]) {
		cut--
	}
	return line[:cut] + "..."
}

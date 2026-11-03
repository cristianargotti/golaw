package check_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/check"
)

const testModule = "example.test/m"

// event is one go test -json line.
func event(action, pkg, test, output string) string {
	data, err := json.Marshal(map[string]string{"Action": action, "Package": pkg, "Test": test, "Output": output})
	if err != nil {
		panic(err)
	}
	return string(data)
}

func stream(lines ...string) *strings.Reader {
	return strings.NewReader(strings.Join(lines, "\n") + "\n")
}

func dirOf(pkg string) string {
	if pkg == testModule {
		return ""
	}
	return strings.TrimPrefix(pkg, testModule+"/")
}

func TestTestFailures(t *testing.T) {
	t.Parallel()
	sub := testModule + "/internal/sub"
	long := strings.Repeat("x", 300)
	for _, tc := range []struct {
		name   string
		events []string
		want   []string
	}{
		{"red-a-failed-test-names-its-file-from-the-module", []string{
			event("run", sub, "TestAdd", ""),
			event("output", sub, "TestAdd", "=== RUN   TestAdd\n"),
			event("output", sub, "TestAdd", "    add_test.go:7: Add = -1, want 3\n"),
			event("output", sub, "TestAdd", "        got more detail\n"),
			event("output", sub, "TestAdd", "--- FAIL: TestAdd (0.00s)\n"),
			event("fail", sub, "TestAdd", ""),
			event("output", sub, "", "FAIL\n"),
			event("output", sub, "", "FAIL\t"+sub+"\t0.1s\n"),
			event("fail", sub, "", ""),
		}, []string{"TestAdd: internal/sub/add_test.go:7: Add = -1, want 3 got more detail"}},
		{"red-only-the-failed-leaf-of-a-subtest", []string{
			event("output", testModule, "TestTable/red", "    table_test.go:12: wrong\n"),
			event("fail", testModule, "TestTable/red", ""),
			event("output", testModule, "TestTable", "--- FAIL: TestTable (0.00s)\n"),
			event("fail", testModule, "TestTable", ""),
			event("fail", testModule, "", ""),
		}, []string{"TestTable/red: table_test.go:12: wrong"}},
		{"red-at-most-three-messages-of-one-test", []string{
			event("output", testModule, "TestMany", "    a_test.go:1: one\n    a_test.go:2: two\n"),
			event("output", testModule, "TestMany", "    a_test.go:3: three\n    a_test.go:4: four\n"),
			event("fail", testModule, "TestMany", ""),
		}, []string{"TestMany: a_test.go:1: one", "TestMany: a_test.go:2: two", "TestMany: a_test.go:3: three"}},
		{"red-a-panic-and-a-silent-failure", []string{
			event("output", testModule, "TestPanic", "panic: boom\n"),
			event("fail", testModule, "TestPanic", ""),
			event("fail", testModule, "TestSilent", ""),
		}, []string{"TestPanic: panic: boom", "TestSilent: failed"}},
		{"red-a-build-error-and-plain-text", []string{
			`{"ImportPath":"example.test/m [example.test/m.test]","Action":"build-output","Output":"# example.test/m\n"}`,
			`{"ImportPath":"example.test/m [example.test/m.test]","Action":"build-output","Output":"./a.go:3:2: undefined: x\n"}`,
			"go: no such tool",
			event("fail", testModule, "", ""),
		}, []string{"a.go:3:2: undefined: x", "go: no such tool", testModule + ": failed"}},
		{"red-a-package-that-failed-outside-its-tests", []string{
			event("output", sub, "", "panic: test timed out after 10m0s\n"),
			event("output", sub, "", "coverage: 10.0% of statements\n"),
			event("fail", sub, "", ""),
		}, []string{sub + ": panic: test timed out after 10m0s"}},
		{"red-a-long-message-is-clipped", []string{
			event("output", testModule, "TestLong", "    a_test.go:1: "+long+"\n"),
			event("fail", testModule, "TestLong", ""),
		}, []string{"TestLong: a_test.go:1: " + strings.Repeat("x", 240-len("TestLong: a_test.go:1: ")) + "..."}},
		{"near-passed-tests-answer-nothing", []string{
			event("output", testModule, "TestOK", "    a_test.go:1: a log line\n"),
			event("pass", testModule, "TestOK", ""),
			event("output", testModule, "", "ok  \texample.test/m\t0.1s\n"),
			event("pass", testModule, "", ""),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := check.TestFailures(stream(tc.events...), dirOf)
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("failures = %q, %v\nwant %q", got, err, tc.want)
			}
		})
	}
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.ErrUnsupported }

func TestTestFailuresReportsAStreamItCannotRead(t *testing.T) {
	t.Parallel()
	if _, err := check.TestFailures(brokenReader{}, dirOf); !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("err = %v", err)
	}
}

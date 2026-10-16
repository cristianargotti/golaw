package main

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

// stream is a module graph: the main module, a module old enough, one
// published the day before, one without a publication time, one replaced by
// a published module and one replaced by a local directory.
const stream = `{"Path": "example.test/kit", "Main": true}
{"Path": "example.test/old", "Version": "v1.2.0", "Time": "2026-09-01T10:00:00Z"}
{"Path": "example.test/young", "Version": "v1.0.1", "Time": "2026-10-13T10:00:00Z"}
{"Path": "example.test/unknown", "Version": "v0.3.0"}
{"Path": "example.test/forked", "Version": "v1.0.0", "Replace": {"Path": "example.test/fork", "Version": "v1.0.2", "Time": "2026-08-01T00:00:00Z"}}
{"Path": "example.test/local", "Version": "v1.0.0", "Replace": {"Path": "../local"}}
`

// today is the clock every judgment runs under.
func today() time.Time { return time.Date(2026, time.October, 14, 12, 0, 0, 0, time.UTC) }

func TestDecodeReadsEveryModule(t *testing.T) {
	t.Parallel()
	modules, err := decode([]byte(stream))
	if err != nil || len(modules) != 6 || !modules[0].Main || modules[4].Replace.Version != "v1.0.2" {
		t.Fatalf("%+v %v", modules, err)
	}
	if _, err := decode([]byte(`{"Path": `)); !errors.Is(err, ErrStream) {
		t.Fatalf("a truncated stream decoded: %v", err)
	}
}

func TestJudgeNamesEveryModuleThatEnteredTooEarly(t *testing.T) {
	t.Parallel()
	modules, err := decode([]byte(stream))
	if err != nil {
		t.Fatal(err)
	}
	lines, judged := judge(modules, today(), defaultDays*day, nil)
	want := []string{
		"example.test/young@v1.0.1 was published 2026-10-13, less than 7 days ago",
		"example.test/unknown@v0.3.0 has no publication time",
		"../local@ has no publication time",
	}
	if judged != 5 || strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("%d %q", judged, lines)
	}
	lines, _ = judge(modules, today(), defaultDays*day,
		[]string{"example.test/young@v1.0.1", "example.test/unknown@v0.3.0", "../local@", "example.test/gone@v1.0.0"})
	if len(lines) != 1 || lines[0] != "-allow example.test/gone@v1.0.0 names no module of the graph" {
		t.Fatalf("%q", lines)
	}
	if lines, _ = judge(modules[:2], today(), defaultDays*day, nil); len(lines) != 0 {
		t.Fatalf("an old module was refused: %q", lines)
	}
}

// listing answers the stream, or the error, the go command would.
func listing(body string, err error) lister {
	return func(context.Context) ([]byte, error) { return []byte(body), err }
}

func TestExecuteMapsTheJudgmentToTheExitCode(t *testing.T) {
	t.Parallel()
	old := `{"Path": "example.test/kit", "Main": true}` + "\n" +
		`{"Path": "example.test/old", "Version": "v1.2.0", "Time": "2026-09-01T10:00:00Z"}` + "\n"
	cases := map[string]struct {
		args []string
		list lister
		code int
		out  string
	}{
		"old enough":     {nil, listing(old, nil), exitClean, "deps-age: 1 judged, each published at least 7 days ago\n"},
		"entered early":  {[]string{"-days", "60"}, listing(old, nil), exitYoung, "less than 60 days ago"},
		"allowed":        {[]string{"-days", "60", "-allow", "example.test/old@v1.2.0"}, listing(old, nil), exitClean, "1 judged"},
		"unknown flag":   {[]string{"-unknown"}, listing(old, nil), exitFailure, ""},
		"positional":     {[]string{"extra"}, listing(old, nil), exitFailure, ""},
		"no minimum":     {[]string{"-days", "0"}, listing(old, nil), exitFailure, ""},
		"go list failed": {nil, listing("", ErrStream), exitFailure, ""},
		"bad stream":     {nil, listing("{", nil), exitFailure, ""},
	}
	for name, tc := range cases {
		var stdout, stderr bytes.Buffer
		code := execute(t.Context(), tc.args, &stdout, &stderr, tc.list, today)
		if code != tc.code || !strings.Contains(stdout.String(), tc.out) {
			t.Errorf("%s: %d %q %q", name, code, stdout.String(), stderr.String())
		}
	}
}

// failingWriter refuses every write.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, ErrStream }

func TestExecuteReportsAFailedWrite(t *testing.T) {
	t.Parallel()
	young := `{"Path": "example.test/young", "Version": "v1.0.1", "Time": "2026-10-13T10:00:00Z"}`
	if code := execute(t.Context(), nil, failingWriter{}, failingWriter{}, listing(young, nil), today); code != exitFailure {
		t.Fatalf("young: %d", code)
	}
	if code := execute(t.Context(), nil, failingWriter{}, failingWriter{}, listing("", nil), today); code != exitFailure {
		t.Fatalf("clean: %d", code)
	}
	if code := execute(t.Context(), nil, failingWriter{}, failingWriter{}, listing("", ErrStream), today); code != exitFailure {
		t.Fatalf("failed list: %d", code)
	}
}

func TestGoListPrintsTheGraphOfTheKit(t *testing.T) {
	t.Parallel()
	body, err := goList(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	modules, err := decode(body)
	if err != nil || len(modules) == 0 || !modules[0].Main || modules[0].Path != "github.com/cristianargotti/golaw" {
		t.Fatalf("%+v %v", modules, err)
	}
	// A listing of the main module alone would judge nothing and pass, so
	// the graph must hold every module the kit requires directly.
	for _, required := range []string{"github.com/spf13/cobra", "gopkg.in/yaml.v3"} {
		if !slices.ContainsFunc(modules, func(m module) bool { return m.Path == required && !m.Main }) {
			t.Fatalf("the graph lacks %s: %+v", required, modules)
		}
	}
	if _, judged := judge(modules, today(), defaultDays*day, nil); judged == 0 {
		t.Fatal("the graph of the kit judged no module")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := goList(ctx); !errors.Is(err, ErrStream) {
		t.Fatalf("a cancelled listing passed: %v", err)
	}
}

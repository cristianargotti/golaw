package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/cristianargotti/golaw/internal/runner"
)

var errCannotStart = errors.New("cannot start")

type scanCase struct {
	name, stream, waivers string
	scannerCode, want     int
	start                 bool
}

func (tc scanCase) read(t *testing.T, command runner.Command, consume func(io.Reader) error) (runner.Output, error) {
	t.Helper()
	if command.Name != "govulncheck" || !slices.Equal(command.Args, []string{"-format", "json", "./..."}) {
		t.Fatalf("command = %+v", command)
	}
	if command.Dir != "" || command.Stdin != nil || !slices.Equal(command.Env, runner.UserEnv(os.Environ())) {
		t.Fatal("scanner must inherit the working directory and use the sanitized user environment")
	}
	if tc.start {
		return runner.Output{}, errCannotStart
	}
	if err := consume(strings.NewReader(tc.stream)); err != nil {
		return runner.Output{}, err
	}
	return runner.Output{Code: tc.scannerCode}, nil
}

func TestScanModeKeepsScannerFailuresAndWaivers(t *testing.T) {
	t.Parallel()
	const clean = `{"config":{"scanner_version":"v1.7.0"}}` + "\n"
	const reached = clean +
		`{"finding":{"osv":"GO-2026-0002","trace":[{"module":"stdlib","function":"Serve"}]}}` + "\n"
	const waived = "vulnerability_waivers:\n" +
		"  - {id: GO-2026-0002, until: 2026-10-20, record: docs/adr/0001-risk.md}\n"
	for _, tc := range []scanCase{
		{"near clean", clean, "", 0, 0, false},
		{"red reachable", reached, "", 0, 1, false},
		{"near waived on last UTC day", reached, waived, 0, 0, false},
		{"red incomplete", "", "", 0, 2, false},
		{"red truncated", clean + "{", "", 0, 2, false},
		{"red scanner exit", clean, "", 2, 2, false},
		{"red cannot start", "", "", 0, 2, true},
		{"red wrong pin", `{"config":{"scanner_version":"v1.6.0"}}`, "", 0, 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			called := false
			deps := scanDependencies{
				now: time.Date(2026, 10, 20, 23, 0, 0, 0, time.UTC),
				readOutput: func(_ context.Context, command runner.Command, consume func(io.Reader) error) (runner.Output, error) {
					called = true
					return tc.read(t, command, consume)
				},
			}
			var stdout, stderr bytes.Buffer
			args := []string{"-scan", "-waivers", waiversFile(t, tc.waivers)}
			got := executeWithScan(t.Context(), args, strings.NewReader("unused"), &stdout, &stderr, deps)
			if !called || got != tc.want {
				t.Fatalf("called %v, exit %d, stdout %q, stderr %q", called, got, stdout.String(), stderr.String())
			}
			if tc.want == 2 && (stdout.Len() != 0 || stderr.Len() == 0) {
				t.Fatal("scanner failure produced a verdict or omitted its reason")
			}
		})
	}
}

// TestScanModeUsesExplicitDateBeforeItsClock holds the date a waiver is
// judged on: a reachable advisory waived until 2026-10-20 is waived under an
// explicit -today of that day, and found under the later clock main supplies
// when -today is absent.
func TestScanModeUsesExplicitDateBeforeItsClock(t *testing.T) {
	t.Parallel()
	const input = `{"config":{"scanner_version":"v1.7.0"}}` + "\n" +
		`{"finding":{"osv":"GO-2026-0002","trace":[{"module":"stdlib","function":"Serve"}]}}` + "\n"
	const policy = "vulnerability_waivers:\n" +
		"  - {id: GO-2026-0002, until: 2026-10-20, record: docs/adr/0001-risk.md}\n"
	deps := scanDependencies{
		now: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		readOutput: func(_ context.Context, _ runner.Command, consume func(io.Reader) error) (runner.Output, error) {
			return runner.Output{}, consume(strings.NewReader(input))
		},
	}
	waivers := waiversFile(t, policy)
	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{"near explicit unexpired date", []string{"-scan", "-today", "2026-10-20", "-waivers", waivers}, 0},
		{"red later clock", []string{"-scan", "-waivers", waivers}, 1},
	} {
		var stdout, stderr bytes.Buffer
		if got := executeWithScan(t.Context(), tc.args, strings.NewReader(""), &stdout, &stderr, deps); got != tc.want {
			t.Fatalf("%s: exit = %d, want %d: %s", tc.name, got, tc.want, stderr.String())
		}
	}
}

func TestScanModeRequiresACompleteBoundedStream(t *testing.T) {
	t.Parallel()
	const clean = `{"config":{"scanner_version":"v1.7.0"}}`
	boundary := clean + strings.Repeat(" ", runner.MaxOutputBytes-len(clean))
	for _, tc := range []struct {
		name  string
		input io.Reader
		want  int
	}{
		{"exact bound", strings.NewReader(boundary), exitClean},
		{"over bound", strings.NewReader(boundary + " "), exitFailure},
		{"read failure", iotest.ErrReader(io.ErrUnexpectedEOF), exitFailure},
		{"bound read failure", io.MultiReader(strings.NewReader(boundary), iotest.ErrReader(io.ErrUnexpectedEOF)), exitFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			deps := scanDependencies{
				now: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC),
				readOutput: func(_ context.Context, _ runner.Command, consume func(io.Reader) error) (runner.Output, error) {
					return runner.Output{}, consume(tc.input)
				},
			}
			var stdout, stderr bytes.Buffer
			args := []string{"-scan", "-waivers", waiversFile(t, "")}
			if got := executeWithScan(t.Context(), args, nil, &stdout, &stderr, deps); got != tc.want {
				t.Fatalf("exit %d, stderr %q", got, stderr.String())
			}
			if tc.want == exitFailure && (stdout.Len() != 0 || stderr.Len() == 0) {
				t.Fatal("unreliable output produced a verdict or omitted its reason")
			}
		})
	}
}

func TestScanModeRejectsCancellationBeforeOrAfterReading(t *testing.T) {
	t.Parallel()
	for _, cancelFirst := range []bool{false, true} {
		ctx, cancel := context.WithCancel(t.Context())
		if cancelFirst {
			cancel()
		}
		called := false
		deps := scanDependencies{
			readOutput: func(_ context.Context, _ runner.Command, consume func(io.Reader) error) (runner.Output, error) {
				called = true
				cancel()
				return runner.Output{}, consume(strings.NewReader(`{"config":{"scanner_version":"v1.7.0"}}`))
			},
		}
		found, err := scanFindings(ctx, deps)
		cancel()
		if !errors.Is(err, context.Canceled) || !errors.Is(err, ErrScanner) || len(found) != 0 || called == cancelFirst {
			t.Fatalf("cancel first %v: called %v, findings %+v, error %v", cancelFirst, called, found, err)
		}
	}
}

type refusedWriter struct{}

func (refusedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestScanModeKeepsOutputFailuresRed(t *testing.T) {
	t.Parallel()
	deps := scanDependencies{
		now: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC),
		readOutput: func(_ context.Context, _ runner.Command, consume func(io.Reader) error) (runner.Output, error) {
			return runner.Output{}, consume(strings.NewReader(`{"config":{"scanner_version":"v1.7.0"}}`))
		},
	}
	args := []string{"-scan", "-waivers", waiversFile(t, "")}
	var stderr bytes.Buffer
	if got := executeWithScan(t.Context(), args, nil, refusedWriter{}, &stderr, deps); got != exitFailure || stderr.Len() == 0 {
		t.Fatalf("output failure: exit %d, stderr %q", got, stderr.String())
	}
	if got := executeWithScan(t.Context(), args, nil, refusedWriter{}, refusedWriter{}, deps); got != exitFailure {
		t.Fatalf("both outputs failed: exit %d", got)
	}
}

func TestScanOptionsUseTheUTCDateAndRejectInvalidDates(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 20, 23, 30, 0, 0, time.FixedZone("west", -5*60*60))
	opts, err := parseOptions([]string{"-scan"}, io.Discard, now)
	if err != nil || opts.day.Format(time.DateOnly) != "2026-10-21" || opts.day.Location() != time.UTC {
		t.Fatalf("UTC day = %s, error %v", opts.day, err)
	}
	if _, err := parseOptions([]string{"-scan", "-today", "tomorrow"}, io.Discard, now); err == nil {
		t.Fatal("explicit invalid date accepted")
	}
}

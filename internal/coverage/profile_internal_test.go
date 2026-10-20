package coverage

import (
	"bufio"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"
)

func TestProfilePreservesModesAndAnyRunCoverage(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"set", "count", "atomic"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			for _, counts := range []string{"0\nfile.go:1.1,2.2 3 7", "7\nfile.go:1.1,2.2 3 0"} {
				text := "\n mode: " + mode + "\n\nfile.go:1.1,2.2 3 " + counts + "\n\t\n"
				files, err := parseProfile(strings.NewReader(text))
				if err != nil || len(files) != 1 || len(files["file.go"].blocks) != 1 {
					t.Fatalf("merged files=%v error=%v", files, err)
				}
				total, err := files["file.go"].totals()
				if err != nil || total != (tally{stmts: 3, covered: 3}) {
					t.Fatalf("merged total=%+v error=%v", total, err)
				}
			}
		})
	}
}

func TestProfileRejectsNegativeAndContradictoryCounts(t *testing.T) {
	t.Parallel()
	for _, text := range []string{
		"file.go:1.1,2.2 -1 0",
		"file.go:1.1,2.2 1 -1",
		"file.go:1.1,2.2 1 0\nfile.go:1.1,2.2 2 1",
		"file.go:1.1,2.2 0 0\nfile.go:1.1,2.2 1 1",
		"file.go:01.01,02.02 1 0\nfile.go:1.1,2.2 2 1",
	} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()
			_, err := parseProfile(strings.NewReader("mode: count\n" + text + "\n"))
			if !errors.Is(err, ErrProfile) {
				t.Fatalf("invalid counts accepted: %v", err)
			}
		})
	}
}

func TestProfileRejectsOnlyMalformedNumericSpans(t *testing.T) {
	t.Parallel()
	for _, location := range []string{
		"file.go", ":1.1,2.2", "file.go:", "file.go:x.1,2.2", "file.go:1,2.2",
		"file.go:1.1,2", "file.go:1.1,2.-1", "file.go:1.1,2.2,3.3",
	} {
		t.Run(location, func(t *testing.T) {
			t.Parallel()
			_, err := parseProfile(strings.NewReader(location + " 1 1\n"))
			if !errors.Is(err, ErrProfile) {
				t.Fatalf("invalid span accepted: %v", err)
			}
		})
	}
	for _, span := range []string{"0.0,0.0", "1.1,1.1", "2.2,1.1", "1.1,2.0"} {
		if _, err := parseProfile(strings.NewReader("file.go:" + span + " 0 0\n")); err != nil {
			t.Fatalf("Go-compatible span %q refused: %v", span, err)
		}
	}
}

func TestProfileKeepsColonInTheSourceName(t *testing.T) {
	t.Parallel()
	name := "C:/source/file.go"
	files, err := parseProfile(strings.NewReader(name + ":1.1,2.2 1 1\n"))
	if err != nil || files[name] == nil || len(files) != 1 {
		t.Fatalf("source identity=%v error=%v", files, err)
	}
}

func TestProfilePreservesReaderCausesAndTheScannerBound(t *testing.T) {
	t.Parallel()
	_, err := parseProfile(io.MultiReader(strings.NewReader("mode: atomic\n"), iotest.ErrReader(io.ErrClosedPipe)))
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("reader error = %v", err)
	}
	_, err = parseProfile(strings.NewReader(strings.Repeat("x", maximumLine)))
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Fatalf("scanner bound = %v", err)
	}
	name := strings.Repeat("x", initialLine+1) + ".go"
	files, err := parseProfile(strings.NewReader(name + ":1.1,2.2 1 0\n"))
	if err != nil || files[name] == nil {
		t.Fatalf("scanner growth = %v", err)
	}
	files, err = parseProfile(strings.NewReader("\n\t\n"))
	if err != nil || len(files) != 0 {
		t.Fatalf("blank profile=%v error=%v", files, err)
	}
}

func TestProfileTotalsRejectStatementOverflow(t *testing.T) {
	t.Parallel()
	maximum := strconv.Itoa(math.MaxInt)
	for _, count := range []string{"0", "1"} {
		text := "file.go:1.1,2.2 " + maximum + " " + count + "\n"
		files, err := parseProfile(strings.NewReader(text))
		if err != nil {
			t.Fatal(err)
		}
		total, err := files["file.go"].totals()
		if err != nil || total.stmts != math.MaxInt {
			t.Fatalf("exact total=%+v error=%v", total, err)
		}
		files, err = parseProfile(strings.NewReader(text + "file.go:3.1,4.2 1 " + count + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := files["file.go"].totals(); !errors.Is(err, ErrProfile) {
			t.Fatalf("overflow accepted: %v", err)
		}
	}
}

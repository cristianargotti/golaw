package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/cristianargotti/golaw/internal/coverage"
)

func TestCoverageRefusesMissingEnrolledPolicy(t *testing.T) {
	t.Parallel()
	for _, missing := range []bool{false, true} {
		ta, repo := coverageCommandFixture(t, 80)
		enrollCoverage(t, ta, repo.Root)
		want := exitClean
		if missing {
			removePolicy(t, repo.Root)
			want = exitFailure
		}
		args := []string{
			"coverage", "--input", filepath.Join(repo.Root, "profile.cov"), repo.Root,
		}
		if got := ta.run(t.Context(), args); got != want {
			t.Fatalf("missing=%t exit=%d stderr=%q", missing, got, ta.stderr.String())
		}
		if missing && !strings.Contains(ta.stderr.String(), "golaw.yml") {
			t.Fatalf("stderr = %q", ta.stderr.String())
		}
	}
}

// enrollCoverage enrolls the golaw.yml of the coverage fixture through the
// binding, as init does.
func enrollCoverage(t *testing.T, ta testApp, root string) {
	t.Helper()
	b := ta.bind()
	data, err := os.ReadFile(filepath.Join(root, "golaw.yml"))
	if err != nil {
		t.Fatal(err)
	}
	effective, err := b.compile(data, now())
	if err != nil {
		t.Fatal(err)
	}
	if enrollErr := b.enroll(root, data, effective, now()); enrollErr != nil {
		t.Fatal(enrollErr)
	}
}

// removePolicy removes the golaw.yml of the coverage fixture once it is
// enrolled.
func removePolicy(t *testing.T, root string) {
	t.Helper()
	if err := os.Remove(filepath.Join(root, "golaw.yml")); err != nil {
		t.Fatal(err)
	}
}

func TestCoverageInputFailures(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"unknown-flag", "absent-module", "broken-profile"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			ta, repo := coverageCommandFixture(t, 80)
			args := []string{
				"coverage", "--input", filepath.Join(repo.Root, "profile.cov"),
			}
			switch kind {
			case "unknown-flag":
				args = append(args, "--unknown")
			case "absent-module":
				if err := os.Remove(filepath.Join(repo.Root, "go.mod")); err != nil {
					t.Fatal(err)
				}
			case "broken-profile":
				repo.Write("profile.cov", "invalid\n")
			}
			args = append(args, repo.Root)
			if got := ta.run(t.Context(), args); got != exitFailure ||
				ta.stderr.Len() == 0 {
				t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
			}
		})
	}
}

type coverageWriteFailure struct{}

func (coverageWriteFailure) Write([]byte) (int, error) {
	return 0, errBoom
}

func TestCoveragePropagatesOutputFailure(t *testing.T) {
	t.Parallel()
	ta, repo := coverageCommandFixture(t, 80)
	ta.app.stdout = coverageWriteFailure{}
	args := []string{
		"coverage", "--input", filepath.Join(repo.Root, "profile.cov"), repo.Root,
	}
	if got := ta.run(t.Context(), args); got != exitFailure {
		t.Fatalf("exit = %d", got)
	}
	if !strings.Contains(ta.stderr.String(), "boom") {
		t.Fatalf("stderr = %q", ta.stderr.String())
	}
}

func TestCoverageKeepsInputRelativeToInvocationDirectory(t *testing.T) { //nolint:paralleltest // t.Chdir changes the process working directory
	ta, repo := coverageCommandFixture(t, 80)
	repo.Write("nested/keep.txt", "coverage repository subdirectory\n")
	t.Chdir(filepath.Join(repo.Root, "nested"))
	args := []string{"coverage", "--input", filepath.Join("..", "profile.cov"), "."}
	if got := ta.run(t.Context(), args); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	if !strings.Contains(ta.stdout.String(), "total: 80.0% (16/20)") {
		t.Fatalf("root measurement = %q", ta.stdout.String())
	}
}

func TestCoverageRefusesAMismatchingPin(t *testing.T) {
	t.Parallel()
	ta, repo := coverageCommandFixture(t, 80)
	b := ta.bind()
	b.info.Version = "1.0.0"
	ta.bind = func() binding { return b }
	args := []string{
		"coverage", "--input", filepath.Join(repo.Root, "profile.cov"), repo.Root,
	}
	if got := ta.run(t.Context(), args); got != exitFailure {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	if !strings.Contains(ta.stderr.String(), "pins kit version") {
		t.Fatalf("pin diagnostic = %q", ta.stderr.String())
	}
	if ta.stdout.Len() != 0 {
		t.Fatalf("pin failure produced a report: %q", ta.stdout.String())
	}
}

type coverageClosingReader struct {
	io.Reader

	closed bool
}

func (r *coverageClosingReader) Close() error {
	r.closed = true
	return errBoom
}

func TestCoverageRetainsReadAndCloseFailures(t *testing.T) {
	t.Parallel()
	_, repo := coverageCommandFixture(t, 80)
	for _, tc := range []struct {
		name   string
		reader io.Reader
		cause  error
	}{
		{"close", strings.NewReader("mode: atomic\n" + commandModule + "/value.go:4.2,23.10 20 1\n"), nil},
		{"parse-and-close", strings.NewReader("invalid\n"), coverage.ErrProfile},
		{"read-and-close", iotest.ErrReader(io.ErrUnexpectedEOF), io.ErrUnexpectedEOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			profile := &coverageClosingReader{Reader: tc.reader}
			_, err := readCoverageProfile(profile, repo.Root, coverage.DefaultFloors())
			if !profile.closed || !errors.Is(err, errBoom) {
				t.Fatalf("closed = %t, error = %v", profile.closed, err)
			}
			if tc.cause != nil && !errors.Is(err, tc.cause) {
				t.Fatalf("error = %v, missing cause %v", err, tc.cause)
			}
		})
	}
}

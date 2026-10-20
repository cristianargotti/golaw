package runner

import (
	"bufio"
	"context"
	"errors"
	"io"
	"math"
	"slices"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

func TestBlobHeaderRequiresTheFrozenIdentityAndSize(t *testing.T) {
	t.Parallel()
	object := strings.Repeat("a", 40)
	for _, tc := range []struct {
		name   string
		header string
		size   int64
		bad    bool
	}{
		{"empty-blob", object + " blob 0\n", 0, false},
		{"ordinary-blob", object + " blob 7\n", 7, false},
		{"wrong-object", strings.Repeat("b", 40) + " blob 7\n", 0, true},
		{"non-blob", object + " tree 7\n", 0, true},
		{"missing-object", object + " missing\n", 0, true},
		{"negative", object + " blob -1\n", 0, true},
		{"signed", object + " blob +1\n", 0, true},
		{"leading-zero", object + " blob 01\n", 0, true},
		{"size-overflow", object + " blob 9223372036854775808\n", 0, true},
		{"truncated", object + " blob 7", 0, true},
		{"extra-field", object + " blob 7 extra\n", 0, true},
		{"oversized", strings.Repeat("x", treeHeaderLimit+1) + "\n", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reader := bufio.NewReader(strings.NewReader(tc.header))
			size, err := readBlobHeader(reader, object)
			if tc.bad {
				if !errors.Is(err, model.ErrCannotJudge) {
					t.Fatalf("header size=%d error=%v", size, err)
				}
				return
			}
			if err != nil || size != tc.size {
				t.Fatalf("header size=%d error=%v, want %d", size, err, tc.size)
			}
		})
	}
}

func TestBlobHeaderBoundsLargerReaderBuffers(t *testing.T) {
	t.Parallel()
	text := strings.Repeat("x", treeHeaderLimit+1) + "\n"
	reader := bufio.NewReaderSize(strings.NewReader(text), 2*treeHeaderLimit)
	if _, err := readBlobHeader(reader, "object"); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatalf("oversized header = %v", err)
	}
}

func TestTreeStreamRejectsTrailingDataAndPreservesReaderErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		src  io.Reader
		want error
	}{
		{"end", strings.NewReader(""), nil},
		{"trailing-newline", strings.NewReader("\n"), model.ErrCannotJudge},
		{"extra-record", strings.NewReader("object blob 0\n"), model.ErrCannotJudge},
		{"reader-error", iotest.ErrReader(io.ErrClosedPipe), io.ErrClosedPipe},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := finishTreeStream(bufio.NewReader(tc.src))
			if !errors.Is(err, tc.want) {
				t.Fatalf("stream end = %v, want %v", err, tc.want)
			}
		})
	}
	_, err := readBlobHeader(bufio.NewReader(iotest.ErrReader(io.ErrClosedPipe)), "object")
	if !errors.Is(err, model.ErrCannotJudge) || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("header reader error = %v", err)
	}
}

func TestTreeSizeAdditionRefusesOverflowBeforeWriting(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name               string
		total, size, limit int64
		want               error
	}{
		{"exact", 2, 2, 4, nil},
		{"over-limit", 2, 3, 4, ErrBound},
		{"int64-overflow", math.MaxInt64, math.MaxInt64, math.MaxInt64, ErrBound},
		{"negative-total", -1, 0, 1, model.ErrCannotJudge},
		{"negative-size", 0, -1, 1, model.ErrCannotJudge},
		{"negative-limit", 0, 0, -1, model.ErrCannotJudge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			total, err := addTreeSize(tc.total, tc.size, tc.limit)
			if !errors.Is(err, tc.want) || (err == nil && total != tc.total+tc.size) {
				t.Fatalf("size total=%d error=%v", total, err)
			}
			if tc.name == "int64-overflow" && !strings.Contains(err.Error(), "18446744073709551614") {
				t.Fatalf("observed overflow size missing: %v", err)
			}
		})
	}
}

func TestTreeObjectInputOmitsGitlinksAndKeepsDuplicates(t *testing.T) {
	t.Parallel()
	entries := []treeEntry{
		{mode: "100644", object: "first"},
		{mode: "160000", object: "missing"},
		{mode: "120000", object: "first"},
	}
	input, err := io.ReadAll(treeObjectInput(entries))
	if err != nil || string(input) != strings.Repeat("first\n", 2) {
		t.Fatalf("object input=%q error=%v", input, err)
	}
}

func TestSizeTreeChecksTheCompleteMetadataStream(t *testing.T) {
	object := strings.Repeat("a", 40)
	valid := object + " blob 3\n" + object + " blob 3\n"
	for _, tc := range []struct {
		name, output string
		bound        int64
		code         int
		want         error
	}{
		{"exact", valid, 6, 0, nil},
		{"duplicate-payload-bound", valid, 5, 0, ErrBound},
		{"truncated", object + " blob 3\n", 6, 0, model.ErrCannotJudge},
		{"additional", valid + "\n", 6, 0, model.ErrCannotJudge},
		{"failed-command", valid, 6, 2, model.ErrCannotJudge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			runnertest.Install(t, dir, "git", runnertest.Behavior{Stdout: tc.output, Code: tc.code})
			t.Setenv("PATH", dir)
			entries := []treeEntry{
				{mode: "100644", object: object},
				{mode: "160000", object: strings.Repeat("f", 40)},
				{mode: "120000", object: object},
			}
			in := MaterializeInput{Dir: t.TempDir(), Bounds: Bounds{Bytes: tc.bound, Files: 3}}
			err := sizeTree(t.Context(), in, entries)
			if !errors.Is(err, tc.want) {
				t.Fatalf("tree sizing error=%v, want %v", err, tc.want)
			}
			if err == nil && (entries[0].size != 3 || entries[1].size != 0 || entries[2].size != 3) {
				t.Fatalf("tree sizes = %+v", entries)
			}
			want := [][]string{{"-c", "core.quotePath=false", "cat-file", "--batch-check"}}
			if calls := runnertest.Calls(t, dir, "git"); !slices.EqualFunc(calls, want, slices.Equal) {
				t.Fatalf("metadata calls = %v", calls)
			}
		})
	}
}

func TestSizeTreePreservesStartupAndCancellation(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	entries := []treeEntry{{mode: "100644", object: strings.Repeat("a", 40)}}
	in := MaterializeInput{Bounds: Bounds{Bytes: 1, Files: 1}}
	if err := sizeTree(t.Context(), in, entries); !errors.Is(err, ErrStart) {
		t.Fatalf("startup error = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := sizeTree(ctx, in, entries); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
	if err := sizeTree(t.Context(), in, []treeEntry{{mode: "160000", object: "absent"}}); err != nil {
		t.Fatalf("gitlink-only tree started git: %v", err)
	}
}

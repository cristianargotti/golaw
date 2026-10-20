package runner_test

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
)

type brokenRefReader struct{}

func (brokenRefReader) Read([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestParseRefUpdates(t *testing.T) {
	t.Parallel()
	a, b := strings.Repeat("a", 40), strings.Repeat("b", 40)
	long, zeroLong := strings.Repeat("c", 64), strings.Repeat("0", 64)
	line := "refs/heads/topic " + a + " refs/heads/topic " + b
	deleted := "(delete) " + runner.ZeroSHA + " refs/heads/gone " + b
	for _, tc := range []struct {
		name  string
		input string
		want  []runner.RefUpdate
		bad   bool
	}{
		{"near-empty", "", nil, false},
		{"near-one", line + "\n", []runner.RefUpdate{
			{LocalRef: "refs/heads/topic", LocalSHA: a, RemoteRef: "refs/heads/topic", RemoteSHA: b},
		}, false},
		{"red-every-pair", line + "\n" + deleted + "\r\n", []runner.RefUpdate{
			{LocalRef: "refs/heads/topic", LocalSHA: a, RemoteRef: "refs/heads/topic", RemoteSHA: b},
			{LocalRef: "(delete)", LocalSHA: runner.ZeroSHA, RemoteRef: "refs/heads/gone", RemoteSHA: b},
		}, false},
		{"near-sha256", "HEAD " + long + " refs/heads/new " + zeroLong, []runner.RefUpdate{
			{LocalRef: "HEAD", LocalSHA: long, RemoteRef: "refs/heads/new", RemoteSHA: zeroLong},
		}, false},
		{"red-three-fields", "HEAD " + a + " refs/heads/new\n", nil, true},
		{"red-five-fields", line + " extra\n", nil, true},
		{"red-short-object", "HEAD abc refs/heads/new " + runner.ZeroSHA, nil, true},
		{"red-nonhex", "HEAD " + strings.Repeat("z", 40) + " refs/heads/new " + b, nil, true},
		{"near-blank-line", " \t\r\n", nil, false},
		{"near-blank-between-updates", "\n" + line + "\n \t\n" + deleted, []runner.RefUpdate{
			{LocalRef: "refs/heads/topic", LocalSHA: a, RemoteRef: "refs/heads/topic", RemoteSHA: b},
			{LocalRef: "(delete)", LocalSHA: runner.ZeroSHA, RemoteRef: "refs/heads/gone", RemoteSHA: b},
		}, false},
		{"red-second-line", line + "\nbroken\n", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := runner.ParseRefUpdates(strings.NewReader(tc.input))
			if tc.bad {
				if !errors.Is(err, runner.ErrRefUpdate) || len(got) != 0 {
					t.Fatalf("updates = %+v, error = %v", got, err)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("updates = %+v, %v", got, err)
			}
		})
	}
	_, err := runner.ParseRefUpdates(brokenRefReader{})
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("reader error = %v", err)
	}
}

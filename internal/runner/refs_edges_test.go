package runner_test

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
)

func TestParseRefUpdatesNamesThePhysicalLine(t *testing.T) {
	t.Parallel()
	valid := "HEAD " + strings.Repeat("a", 40) + " refs/heads/topic " + runner.ZeroSHA
	broken := "HEAD\tbad-object refs/heads/topic " + runner.ZeroSHA
	input := "\r\n" + valid + "\n \t\r\n" + broken + "\r\n"
	got, err := runner.ParseRefUpdates(strings.NewReader(input))
	if !errors.Is(err, runner.ErrRefUpdate) || len(got) != 0 {
		t.Fatalf("updates = %+v, error = %v", got, err)
	}
	if !strings.Contains(err.Error(), "line 4 ") || !strings.Contains(err.Error(), fmt.Sprintf("%q", broken)) {
		t.Fatalf("error does not preserve the physical line: %v", err)
	}
}

func TestParseRefUpdatesDiscardsUpdatesAfterReaderFailure(t *testing.T) {
	t.Parallel()
	valid := "HEAD " + strings.Repeat("a", 40) + " refs/heads/topic " + runner.ZeroSHA + "\n"
	input := io.MultiReader(strings.NewReader(valid), brokenRefReader{})
	got, err := runner.ParseRefUpdates(input)
	if !errors.Is(err, io.ErrClosedPipe) || len(got) != 0 {
		t.Fatalf("updates = %+v, error = %v", got, err)
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("reader error does not name the failed line: %v", err)
	}
}

func TestParseRefUpdatesAcceptsLongRefExpressions(t *testing.T) {
	t.Parallel()
	local := strings.Repeat("long", 1<<15)
	object := strings.Repeat("A", 40)
	input := local + " " + object + " refs/heads/topic " + runner.ZeroSHA
	got, err := runner.ParseRefUpdates(strings.NewReader(input))
	if err != nil || len(got) != 1 {
		t.Fatalf("updates = %+v, error = %v", got, err)
	}
	if got[0].LocalRef != local || got[0].LocalSHA != object {
		t.Fatalf("update does not preserve the advertised expression: %+v", got[0])
	}
}

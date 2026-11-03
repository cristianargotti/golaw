package runner_test

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
)

// Two processes Start began run at once and each Wait answers what Run
// would: the status, the kept streams, or the caller's writer.
func TestStartRunsProcessesAtOnce(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	var stream bytes.Buffer
	first, err := runner.Start(t.Context(), git(r, "", "version"), &stream)
	if err != nil {
		t.Fatal(err)
	}
	second, err := runner.Start(t.Context(), git(r, "", "no-such-command"), nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := second.Wait(t.Context())
	if err != nil || out.Code == 0 || len(out.Stderr) == 0 {
		t.Errorf("second = %+v, %v", out, err)
	}
	out, err = first.Wait(t.Context())
	if err != nil || out.Code != 0 || len(out.Stdout) != 0 || !strings.HasPrefix(stream.String(), "git version ") {
		t.Errorf("first = %+v, %v, stream %q", out, err, stream.String())
	}
}

func TestStartRefusesWhatCannotStart(t *testing.T) {
	t.Parallel()
	missing := runner.Command{Name: filepath.Join(t.TempDir(), "absent")}
	if _, err := runner.Start(t.Context(), missing, nil); !errors.Is(err, runner.ErrStart) {
		t.Errorf("absent program = %v, want ErrStart", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := runner.Start(ctx, git(gittest.New(t), "", "version"), nil); !errors.Is(err, context.Canceled) {
		t.Errorf("ended context = %v, want its error", err)
	}
}

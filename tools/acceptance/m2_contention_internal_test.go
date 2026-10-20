package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/runner"
)

func m2Contention(t *testing.T, s *suite) string {
	t.Helper()
	installed, err := s.timingM2(t.Context(), "pre-commit", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	where := s.execute(t.Context(), command{
		program: s.git,
		args:    []string{"rev-parse", "--absolute-git-dir"},
		dir:     installed.dir, env: installed.env,
	})
	if whereErr := success(where); whereErr != nil {
		t.Fatal(whereErr)
	}
	path := runner.LockPath(strings.TrimSpace(where.stdout), "pre-commit")
	lock, err := runner.Acquire(t.Context(), path, runner.RungWait)
	if err != nil {
		t.Fatal(err)
	}
	direct := command{
		program: s.binary, args: []string{"hook", "pre-commit"},
		dir: installed.dir, env: installed.env,
	}
	pid := strconv.Itoa(os.Getpid())
	start := time.Now()
	first := s.execute(t.Context(), direct)
	directWait := time.Since(start)
	start = time.Now()
	second := s.execute(t.Context(), installed)
	installedWait := time.Since(start)
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	if first.err != nil || first.code != 2 ||
		!strings.Contains(first.stdout+first.stderr, pid) {
		t.Fatalf("direct contention did not name its holder: %+v", first)
	}
	if second.err != nil || second.code == 0 ||
		!strings.Contains(second.stdout+second.stderr, pid) {
		t.Fatalf("installed contention did not name its holder: %+v", second)
	}
	m2MeasuredResult(t, "pre-commit", s.execute(t.Context(), direct))
	m2MeasuredResult(t, "pre-commit", s.execute(t.Context(), installed))
	return fmt.Sprintf(
		"lock_pid=%s direct_wait_ms=%d direct_exit=%d installed_wait_ms=%d installed_exit=%d",
		pid, directWait.Milliseconds(), first.code,
		installedWait.Milliseconds(), second.code,
	)
}

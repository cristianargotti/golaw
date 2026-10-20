package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

type releasingNotice struct {
	output  bytes.Buffer
	release func() error
}

func (w *releasingNotice) Write(data []byte) (int, error) {
	n, err := w.output.Write(data)
	if err != nil {
		return n, fmt.Errorf("record lock notice: %w", err)
	}
	if w.release != nil {
		release := w.release
		w.release = nil
		if err := release(); err != nil {
			return n, err
		}
	}
	return n, nil
}

//nolint:paralleltest // The command resolves its repository from cwd.
func TestLockCommandNamesTheHolderBeforeWaiting(t *testing.T) {
	repo := gittest.New(t)
	t.Chdir(repo.Root)
	path := runner.LockPath(repo.GitDir(), "measure")
	held, err := runner.Acquire(t.Context(), path, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := held.Release(); err != nil {
			t.Error(err)
		}
	})
	program := runnertest.Install(
		t, t.TempDir(), "probe", runnertest.Behavior{Probe: []string{"version"}},
	)
	notice := &releasingNotice{release: held.Release}
	ta := ladderApp(t)
	ta.app.stderr = notice
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	if got := ta.run(ctx, []string{
		"lock", "measure", "--", program,
	}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, notice.output.String())
	}
	want := fmt.Sprintf(
		"aguardando o processo %d liberar o bloqueio %s\n",
		os.Getpid(), path,
	)
	if notice.output.String() != want {
		t.Fatalf("notice = %q, want %q", notice.output.String(), want)
	}
	if notice.release != nil {
		t.Fatal("command never announced the held lock")
	}
}

func TestLockedBusyFailuresNeverStartTheCommand(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		want   error
		notice func(context.CancelFunc) error
	}{
		{"write-failure", errBoom, func(context.CancelFunc) error { return errBoom }},
		{"canceled", context.Canceled, func(cancel context.CancelFunc) error {
			cancel()
			return nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			path := runner.LockPath(repo.GitDir(), "measure")
			held := holdCommandLock(t, path)
			dir := t.TempDir()
			program := runnertest.Install(t, dir, "probe", runnertest.Behavior{
				Probe: []string{"version"},
			})
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			notice := &releasingNotice{release: func() error {
				return tc.notice(cancel)
			}}
			b := ladderApp(t).bind()
			_, err := b.locked(ctx, repo.Root, "measure",
				[]string{program}, nil, io.Discard, notice)
			if !errors.Is(err, tc.want) || len(runnertest.Calls(t, dir, "probe")) != 0 {
				t.Fatalf("busy command error = %v, want %v", err, tc.want)
			}
			if strings.Count(notice.output.String(), "\n") != 1 || notice.release != nil {
				t.Fatalf("notice = %q", notice.output.String())
			}
			if err := held.Release(); err != nil {
				t.Fatal(err)
			}
			requireLockReleased(t, repo.GitDir())
		})
	}
}

func holdCommandLock(t *testing.T, path string) *runner.Lock {
	t.Helper()
	held, err := runner.Acquire(t.Context(), path, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if releaseErr := held.Release(); releaseErr != nil {
			t.Error(releaseErr)
		}
	})
	return held
}

func TestLockedRejectsAcquisitionFailureWithoutAWaitingNotice(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	path := runner.LockPath(repo.GitDir(), "measure")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	program := runnertest.Install(t, dir, "probe", runnertest.Behavior{
		Probe: []string{"version"},
	})
	var notice bytes.Buffer
	b := ladderApp(t).bind()
	_, err := b.locked(t.Context(), repo.Root, "measure",
		[]string{program}, nil, io.Discard, &notice)
	if !errors.Is(err, os.ErrInvalid) || notice.Len() != 0 ||
		len(runnertest.Calls(t, dir, "probe")) != 0 {
		t.Fatalf("acquisition error = %v, notice = %q", err, notice.String())
	}
}

func TestLockedJoinsOutputAndReleaseFailures(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	path := runner.LockPath(repo.GitDir(), "measure") + ".pid"
	dir := t.TempDir()
	program := runnertest.Install(t, dir, "probe", runnertest.Behavior{
		Probe: []string{"version"}, Stdout: "ready\n",
	})
	output := &releasingNotice{release: func() error {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove PID file: %w", err)
		}
		if err := os.Mkdir(path, 0o700); err != nil {
			return fmt.Errorf("replace PID file: %w", err)
		}
		if err := os.WriteFile(filepath.Join(path, "keep"), nil, 0o600); err != nil {
			return fmt.Errorf("fill PID directory: %w", err)
		}
		return errBoom
	}}
	b := ladderApp(t).bind()
	_, err := b.locked(t.Context(), repo.Root, "measure",
		[]string{program}, nil, output, io.Discard)
	failure, found := errors.AsType[*os.PathError](err)
	if !errors.Is(err, errBoom) || !found || failure.Op != "remove" || failure.Path != path {
		t.Fatalf("joined output and release errors = %v", err)
	}
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	requireLockReleased(t, repo.GitDir())
}

func TestLockedCanceledBeforeDiscoveryNeverStartsTheCommand(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	dir := t.TempDir()
	program := runnertest.Install(t, dir, "probe", runnertest.Behavior{
		Probe: []string{"version"},
	})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	b := ladderApp(t).bind()
	_, err := b.locked(ctx, repo.Root, "measure",
		[]string{program}, nil, io.Discard, io.Discard)
	if !errors.Is(err, context.Canceled) || len(runnertest.Calls(t, dir, "probe")) != 0 {
		t.Fatalf("canceled command = %v", err)
	}
	path := runner.LockPath(repo.GitDir(), "measure")
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled command created a lock: %v", err)
	}
}

func TestDemandLockExpiryNamesTheHolderAndRecovery(t *testing.T) {
	t.Parallel()
	path := runner.LockPath(t.TempDir(), "measure")
	holdCommandLock(t, path)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	var notice bytes.Buffer
	lock, err := demandLock(ctx, path, 20*time.Millisecond, &notice)
	held, found := errors.AsType[*runner.LockedError](err)
	if lock != nil || !errors.Is(err, runner.ErrLocked) || !found ||
		held.PID != os.Getpid() || held.Path != path {
		t.Fatalf("expired acquisition = %v, %v", lock, err)
	}
	want := fmt.Sprintf("espere o processo %d terminar e rode o comando de novo", os.Getpid())
	if !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), path) {
		t.Fatalf("recovery = %q, want %q and %q", err, want, path)
	}
	want = fmt.Sprintf("aguardando o processo %d liberar o bloqueio %s\n", os.Getpid(), path)
	if notice.String() != want {
		t.Fatalf("notice = %q, want %q", notice.String(), want)
	}
}

func TestLockWaitDiagnosticsDescribeKnownAndUnknownHolders(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, notice, instruction string
		pid                       int
	}{
		{
			name: "known", pid: 42,
			notice:      "aguardando o processo 42 liberar o bloqueio measure.lock",
			instruction: "espere o processo 42 terminar e rode o comando de novo",
		},
		{
			name:        "unknown",
			notice:      "aguardando a liberação do bloqueio measure.lock, cujo processo não foi identificado",
			instruction: "espere o processo que segura o bloqueio terminar e rode o comando de novo",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			held := &runner.LockedError{Path: "measure.lock", PID: tc.pid}
			if notice := lockWaitNotice(held); notice != tc.notice {
				t.Fatalf("notice = %q, want %q", notice, tc.notice)
			}
			err := lockWaitFailed(errors.Join(held, errBoom))
			want := "bloqueio measure.lock ocupado; " + tc.instruction
			if err.Error() != want {
				t.Fatalf("recovery = %q, want %q", err, want)
			}
			got, found := errors.AsType[*runner.LockedError](err)
			if !errors.Is(err, runner.ErrLocked) || !errors.Is(err, errBoom) || !found || got != held {
				t.Fatalf("lost acquisition causes: %v", err)
			}
		})
	}
}

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestM2RoutingOnlyPreparesAndCollectsM2(t *testing.T) {
	t.Parallel()
	driver := m2NewRoutingDriver(t)
	var out bytes.Buffer
	if code := runArgs(&out, driver.execute(t), driver.lookup(t), []string{"-m2"}); code != 0 {
		t.Fatalf("M2 route exit=%d\n%s", code, out.String())
	}
	m2RequireRoutingRows(t, out.String())
	if driver.m1Builds != 0 || driver.agentCalls != 0 || driver.agentLookups != 0 || driver.preparations != 1 {
		t.Fatalf("M2 route escaped its lane: %+v", driver)
	}
}

func TestM2RoutingReleaseKeepsM1AndAddsM2(t *testing.T) {
	t.Parallel()
	driver := m2NewRoutingDriver(t)
	var out bytes.Buffer
	code := runArgs(&out, driver.execute(t), driver.lookup(t), []string{"-release"})
	if code != 1 || !strings.Contains(out.String(), "host-timeout") || !strings.Contains(out.String(), "host-missing-binary") {
		t.Fatalf("release lost required M1 evidence: exit=%d\n%s", code, out.String())
	}
	m2RequireRoutingRows(t, out.String())
	// One M1 build, and a second for the candidate digest once the
	// module holds its dated live record.
	if driver.m1Builds < 1 || driver.m1Builds > 2 || driver.agentCalls == 0 || driver.preparations != 1 {
		t.Fatalf("release preparation changed: %+v", driver)
	}
}

func TestM2RoutingPreparationFailureStaysRequired(t *testing.T) {
	t.Parallel()
	driver := m2NewRoutingDriver(t)
	driver.prepareMode = "broken"
	var out bytes.Buffer
	if code := runArgs(&out, driver.execute(t), driver.lookup(t), []string{"-m2"}); code != 1 {
		t.Fatalf("failed candidate passed: exit=%d", code)
	}
	text := out.String()
	if !strings.Contains(text, "m2-preparation\tgit\t") || !strings.Contains(text, "\tFAIL\trequired\tcandidate\t") ||
		strings.Contains(text, "m2-staged") || driver.preparations != 1 {
		t.Fatalf("preparation failure was omitted or collection continued:\n%s", text)
	}
}

func TestM2RoutingRejectsExtraArgumentsBeforePreparation(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"-m2", "unexpected"}, {"-m2", "-unknown"}, {"-release", "unexpected"}} {
		called := false
		execute := func(context.Context, command) result {
			called = true
			return result{err: ErrAcceptance}
		}
		lookup := func(string) (string, error) {
			called = true
			return "", ErrAcceptance
		}
		if code := runArgs(io.Discard, execute, lookup, args); code != 1 || called {
			t.Fatalf("arguments=%q exit=%d prepared=%t", args, code, called)
		}
	}
}

func TestM2RoutingReportWriteFailureFails(t *testing.T) {
	t.Parallel()
	driver := m2NewRoutingDriver(t)
	writer := &m2RowFailureWriter{}
	if code := runArgs(writer, driver.execute(t), driver.lookup(t), []string{"-m2"}); code != 1 || writer.writes != 2 {
		t.Fatalf("report write failure exit=%d writes=%d", code, writer.writes)
	}
	if !driver.prepared || driver.collection.Targets == 0 {
		t.Fatal("writer failure was tested without completing candidate preparation and collection")
	}
}

func TestM2RoutingOptionalCommandTimeout(t *testing.T) {
	t.Parallel()
	if processTimeout != 2*time.Minute {
		t.Fatalf("M1 process timeout changed: %v", processTimeout)
	}
	for _, override := range []time.Duration{0, -time.Second, time.Second} {
		c := command{program: filepath.Join(t.TempDir(), "missing-executable"), timeout: override}
		got := runCommandWithin(t.Context(), c, -time.Second)
		deadline := errors.Is(got.err, context.DeadlineExceeded)
		if got.err == nil || deadline != (override <= 0) {
			t.Fatalf("override=%v result=%+v", override, got)
		}
	}
}

func m2RequireRoutingRows(t *testing.T, text string) {
	t.Helper()
	count := 0
	for line := range strings.SplitSeq(text, "\n") {
		if !strings.HasPrefix(line, "m2-") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 9 || fields[1] != "git" || len(fields[4]) != 64 {
			t.Fatalf("M2 row lost Git or candidate identity: %s", line)
		}
		if fields[7] == m2WindowsOnly && fields[5] == statusSkip && fields[6] == "optional" {
			count++
			continue
		}
		if fields[5] != statusPass || fields[6] != "required" {
			t.Fatalf("M2 functional row did not pass: %s", line)
		}
		count++
	}
	want := len(m2StagedNames()) + len(m2LifecycleNames()) + len(m2NativeNames())
	if count != want {
		t.Fatalf("functional rows=%d want=%d\n%s", count, want, text)
	}
}

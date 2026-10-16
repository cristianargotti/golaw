package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func faultExecutor(t *testing.T, base executor) executor {
	t.Helper()
	return func(ctx context.Context, c command) result {
		if len(c.args) == 0 || c.args[0] != commandHook {
			return base(ctx, c)
		}
		mode, err := readFile(c.program + ".mode")
		if err == nil {
			switch string(mode) {
			case faultCrash:
				return result{code: 2, stderr: "controlled crash\n"}
			case faultEmpty:
				return result{}
			case faultTimeout:
				return result{err: context.DeadlineExceeded}
			}
		}
		first := filepath.SplitList(environmentValue(c.env, envPath))[0]
		if filepath.Base(first) == "slowgit" {
			return result{code: 2, stderr: "internal deadline passed\n"}
		}
		return base(ctx, c)
	}
}

func TestFailureRows(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	s.execute = faultExecutor(t, s.execute)
	rows := s.failureRows(t.Context())
	if len(rows) != 6 || rows[5].name != caseRecovery {
		t.Fatalf("%+v", rows)
	}
	for _, item := range rows {
		assertPass(t, item)
	}
	// Exit 0 with no output is the allow of the host protocol, so the empty
	// row proves the injection and is kept apart from the denial rows.
	for _, item := range rows[:4] {
		want := categoryTransport
		if item.name == faultEmpty {
			want = categoryProtocolLimit
		}
		if item.category != want {
			t.Errorf("%s is in %s, want %s", item.name, item.category, want)
		}
	}
	if !strings.Contains(rows[2].detail, "an allow to the host") || strings.Contains(rows[2].detail, "onFailure") {
		t.Errorf("empty row detail = %q", rows[2].detail)
	}
	body, err := readFile(s.binary)
	if err != nil || string(body) != "simulated executable" {
		t.Fatalf("%q %v", body, err)
	}
}

func TestFailureAssertions(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{faultStartup, faultCrash, faultEmpty, faultTimeout, "unknown"} {
		if faultResult(mode, result{code: 1}) == nil {
			t.Fatal(mode)
		}
	}
	if faultResult(faultStartup, result{err: ErrAcceptance}) != nil ||
		faultResult(faultCrash, result{code: 2, stderr: "crash"}) != nil ||
		faultResult(faultEmpty, result{}) != nil ||
		faultResult(faultTimeout, result{err: context.DeadlineExceeded}) != nil {
		t.Fatal("valid fault rejected")
	}
	if faultResult(faultEmpty, result{stderr: "unexpected"}) == nil {
		t.Fatal("unexpected stderr accepted")
	}
	s := testSuite(t)
	s.execute = func(context.Context, command) result { return result{err: ErrAcceptance} }
	if got := s.failureRows(t.Context()); len(got) != 1 || got[0].status != statusFail {
		t.Fatalf("%+v", got)
	}
	s = testSuite(t)
	if got := s.slowRow(t.Context(), s.binary, s.home, nil); got.status != statusFail {
		t.Fatalf("%+v", got)
	}
	if _, err := s.buildFault(t.Context(), filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("absent fault source passed")
	}
	s = testSuite(t)
	s.execute = func(context.Context, command) result { return result{} }
	if got := s.failureRows(t.Context()); len(got) != 1 || got[0].status != statusFail {
		t.Fatalf("%+v", got)
	}
}

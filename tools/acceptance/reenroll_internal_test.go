package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// reenrollAnswers answers the commands of the re-enrollment row the way
// the real binary does when the repair works, with one answer changed by
// broken; every other command goes to base.
func reenrollAnswers(t *testing.T, base executor, broken string) executor {
	t.Helper()
	installs, scans := 0, 0
	return func(ctx context.Context, c command) result {
		if !strings.HasSuffix(c.dir, caseReenroll) || len(c.args) == 0 {
			return base(ctx, c)
		}
		switch c.args[0] {
		case "agent":
			installs++
			if installs == 2 && broken != "install" {
				return result{stdout: enrollmentStep + "sha256:a becomes sha256:b\n"}
			}
		case "scan":
			scans++
			if scans == 1 && broken != "scan" {
				return result{code: 2, stderr: "golaw: scan: mismatch: run agent install to enroll the policy in force\n"}
			}
		case commandHook:
			return reenrollHook(t, c, broken == "hook")
		}
		return base(ctx, c)
	}
}

// reenrollHook denies the 201 line write of the row and allows the rest,
// or allows everything when the hook is broken.
func reenrollHook(t *testing.T, c command, broken bool) result {
	t.Helper()
	var envelope struct {
		ID string `json:"tool_use_id"`
	}
	if err := json.Unmarshal([]byte(c.input), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.ID == caseReenroll+"-201" && !broken {
		return simulatedAnswer(wantDeny)
	}
	return simulatedAnswer(wantAllow)
}

func TestReenrollRow(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	base := s.execute
	s.execute = reenrollAnswers(t, base, "")
	got := s.reenrollRow(t.Context())
	assertPass(t, got)
	if got.category != categoryInstallation || got.name != caseReenroll {
		t.Fatalf("%+v", got)
	}
	for _, broken := range []string{"install", "scan", "hook"} {
		s.execute = reenrollAnswers(t, base, broken)
		if got := s.reenrollRow(t.Context()); got.status != statusFail {
			t.Errorf("a broken %s passed: %+v", broken, got)
		}
	}
}

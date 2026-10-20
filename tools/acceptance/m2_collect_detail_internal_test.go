package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
)

func TestM2CollectionDetailsAndContinuation(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	s.m2OS, s.m2.Ready = "linux", false
	script := m2CollectionScript(t, s, "answer")
	homes := make(map[string]string)
	cleanSetup, failedTarget, probes, cleanCalls := false, false, 0, 0
	s.execute = func(ctx context.Context, c command) result {
		if c.program == s.git && slices.Equal(c.args, []string{"--version"}) {
			probes++
			return result{stdout: "git version 2.50.0.fixture\n"}
		}
		name := m2CaseName(s, c)
		m2RequireCollectionHome(t, s, homes, c, name)
		m2RequireCollectionTools(t, s, c, name, cleanSetup)
		if name == "partial-red" && slices.Equal(c.args, []string{"scan", "--staged", "--json"}) {
			failedTarget = true
			return result{err: ErrAcceptance}
		}
		got := script.Execute(ctx, c)
		if name == "setup-clean" && m2Kind(s, c) == "setup" {
			cleanCalls++
			cleanSetup = got.err == nil && got.code == 0
		}
		return got
	}
	rows, evidence := s.collectM2(t.Context())
	m2RequireCollectionCatalog(t, rows, evidence, s, homes)
	if !cleanSetup || !failedTarget || probes != 1 || cleanCalls != 1 {
		t.Fatalf("setup=%v target failure=%v Git probes=%d clean setup calls=%d", cleanSetup, failedTarget, probes, cleanCalls)
	}
}

func m2RequireCollectionHome(t *testing.T, s *suite, homes map[string]string, c command, name string) {
	t.Helper()
	if name == "" {
		t.Fatalf("case command lacks an isolated case home: program=%s args=%q", c.program, c.args)
	}
	want := filepath.Join(s.m2.Base, "cases", name, "home")
	if home := environmentValue(c.env, "HOME"); home != want || home == s.home {
		t.Fatalf("%s home=%q want=%q", name, home, want)
	}
	if prior, found := homes[name]; found && prior != want {
		t.Fatalf("%s changed its case home", name)
	}
	homes[name] = want
}

func m2RequireCollectionTools(t *testing.T, s *suite, c command, name string, cleanSetup bool) {
	t.Helper()
	if name == "setup-clean" || m2Kind(s, c) != "probe" {
		return
	}
	if !cleanSetup || !s.m2.Ready {
		t.Fatalf("%s consumed verified tools before setup-clean completed", name)
	}
}

func m2RequireCollectionCatalog(t *testing.T, rows []row, evidence map[string]rowEvidence, s *suite, homes map[string]string) {
	t.Helper()
	names := make([]string, 0, 46)
	for _, group := range []struct {
		category string
		names    []string
	}{
		{"m2-staged", m2StagedNames()},
		{"m2-lifecycle", m2LifecycleNames()},
		{"m2-rung", m2NativeNames()},
	} {
		for _, name := range group.names {
			names = append(names, group.category+"/"+name)
		}
	}
	if len(names) != 46 || len(rows) != len(names) || len(evidence) != len(names) {
		t.Fatalf("catalog=%d rows=%d evidence=%d, want 46", len(names), len(rows), len(evidence))
	}
	body, err := os.ReadFile(s.binary)
	if err != nil {
		t.Fatal(err)
	}
	digest := install.Digest(body)
	for index, item := range rows {
		key := item.category + "/" + item.name
		if key != names[index] || evidence[key] != (rowEvidence{"git", "2.50.0.fixture", digest}) {
			t.Fatalf("row %d=%s evidence=%+v want=%s with actual identity", index, key, evidence[key], names[index])
		}
		m2RequireCollectionOutcome(t, item, homes)
	}
}

func m2RequireCollectionOutcome(t *testing.T, item row, homes map[string]string) {
	t.Helper()
	if item.name == m2WindowsOnly {
		if item.status != statusSkip || item.required || homes[item.name] != "" {
			t.Fatalf("Windows-only case ran or became required off Windows: %+v", item)
		}
		return
	}
	want := statusPass
	if item.name == "partial-red" {
		want = statusFail
	}
	if item.status != want || !item.required || homes[item.name] == "" {
		t.Fatalf("case failure interrupted or contaminated collection: %+v, home=%q", item, homes[item.name])
	}
}

func TestM2CollectionRejectsUnavailableGitIdentity(t *testing.T) {
	t.Parallel()
	for _, got := range []result{
		{err: ErrAcceptance}, {code: 1}, {}, {stdout: "git version unavailable\n"}, {stdout: "not a Git version\n"},
	} {
		s := m2TestSuite(t)
		calls := 0
		s.execute = func(_ context.Context, c command) result {
			calls++
			if c.program != s.git || !slices.Equal(c.args, []string{"--version"}) {
				t.Fatalf("failed Git identity still started a case: %+v", c)
			}
			return got
		}
		rows, evidence := s.collectM2(t.Context())
		if calls != 1 || len(rows) != 1 || rows[0].status != statusFail || !rows[0].required {
			t.Fatalf("Git identity failure was not a required preparation failure: calls=%d rows=%+v", calls, rows)
		}
		for _, identity := range evidence {
			if identity.Version != "" {
				t.Fatalf("failed Git probe fabricated a version: %+v", identity)
			}
		}
	}
}

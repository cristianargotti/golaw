package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// m4Sample is a catalog of two rows, the second skipped, whose skip is
// required when the run requires it.
func m4Sample(agent string, calls *int) m4Catalog {
	return m4Catalog{agent: agent, version: "1.2.3", rows: func(_ *suite, _ context.Context, required bool) []row {
		*calls++
		return []row{
			measuredM4("m4-sample", agent+"-a", nil),
			{"m4-sample", agent + "-b", statusSkip, "absent here", required},
		}
	}}
}

// m4RequireSampleRows holds the rows of two sample catalogs to their order,
// their detail, the required flag of the run and their evidence.
func m4RequireSampleRows(t *testing.T, s *suite, rows []row, evidence map[string]rowEvidence, required bool) {
	t.Helper()
	keys := make([]string, 0, len(rows))
	for _, item := range rows {
		keys = append(keys, item.category+"/"+item.name)
	}
	if !slices.Equal(keys, []string{"m4-sample/first-a", "m4-sample/first-b", "m4-sample/second-a", "m4-sample/second-b"}) {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].detail != m4Verified || rows[1].required != required {
		t.Fatalf("required %v: rows = %+v", required, rows)
	}
	for _, key := range keys {
		agent, _, _ := strings.Cut(strings.TrimPrefix(key, "m4-sample/"), "-")
		if got := evidence[key]; got.Agent != agent || got.Version != "1.2.3" || got.Binary != s.digest {
			t.Fatalf("%s evidence = %+v", key, got)
		}
	}
}

func TestM4CollectCatalogsKeysEveryRowAndItsEvidence(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	calls := 0
	for _, required := range []bool{false, true} {
		rows, evidence := s.m4CollectCatalogs(t.Context(), []m4Catalog{m4Sample("first", &calls), m4Sample("second", &calls)}, required)
		m4RequireSampleRows(t, s, rows, evidence, required)
	}
	if calls != 4 {
		t.Fatalf("catalogs ran %d times, want 4", calls)
	}
	if got := measuredM4("m4-sample", "red", ErrAcceptance); got.status != statusFail || got.detail != ErrAcceptance.Error() {
		t.Fatalf("a failing row = %+v", got)
	}
}

func TestM4RunReportsThePreparationOrTheRows(t *testing.T) {
	t.Parallel()
	calls := 0
	s := testSuite(t)
	var out bytes.Buffer
	if code := s.m4Run(t.Context(), &out, []m4Catalog{m4Sample("first", &calls)}); code != 0 ||
		!strings.Contains(out.String(), "m4-sample\tfirst\t") || !strings.Contains(out.String(), "\tSKIP\toptional\tfirst-b\t") {
		t.Fatalf("a prepared run = %d\n%s", code, out.String())
	}
	failing := newSuite(testRoot(t), t.TempDir(), func(context.Context, command) result { return result{err: ErrAcceptance} },
		func(name string) (string, error) { return name, nil })
	out.Reset()
	if code := failing.m4Run(t.Context(), &out, []m4Catalog{m4Sample("first", &calls)}); code != 1 ||
		!strings.Contains(out.String(), "m4-preparation\tall\t") || !strings.Contains(out.String(), "\tFAIL\trequired\tcandidate\t") {
		t.Fatalf("a failed build = %d\n%s", code, out.String())
	}
	if calls != 1 {
		t.Fatalf("a failed build ran the catalogs: %d", calls)
	}
}

func TestRunArgsRoutesTheM4Flag(t *testing.T) {
	t.Parallel()
	called := false
	failing := func(context.Context, command) result {
		called = true
		return result{err: ErrAcceptance}
	}
	lookup := func(name string) (string, error) { return filepath.Join(t.TempDir(), name), nil }
	var out bytes.Buffer
	if code := runArgs(&out, failing, lookup, []string{"-m4", "unexpected"}); code != 1 || called {
		t.Fatalf("an extra argument prepared the candidate: %d %v", code, called)
	}
	if code := runArgs(&out, failing, lookup, []string{"-m4"}); code != 1 ||
		!strings.Contains(out.String(), "m4-preparation\t") || !strings.Contains(out.String(), "\tFAIL\trequired\tcandidate\t") {
		t.Fatalf("a failed M4 candidate = %d\n%s", code, out.String())
	}
	only := true
	if code, routed := (m4Flags{only: &only}).route(t.Context(), &out, nil, true); routed || code != 0 {
		t.Fatal("a release run routed to the M4 rows alone")
	}
}

func TestM4HomeIsARealPathWithACodexHome(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	home, env, err := s.m4Home("sample")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(home)
	if err != nil || resolved != home {
		t.Fatalf("home %q resolves to %q: %v", home, resolved, err)
	}
	if info, statErr := os.Stat(filepath.Join(home, ".codex")); statErr != nil || !info.IsDir() {
		t.Fatalf("the Codex home is absent: %v", statErr)
	}
	for key, want := range map[string]string{
		"HOME": home, "CODEX_HOME": filepath.Join(home, ".codex"), "XDG_STATE_HOME": filepath.Join(home, "state"),
	} {
		if got := environmentValue(env, key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
	if first := filepath.SplitList(environmentValue(env, envPath))[0]; first != filepath.Dir(s.binary) {
		t.Fatalf("PATH starts with %q", first)
	}
	checkedWrite(t, filepath.Join(s.base, "blocked"), "a file")
	s.base = filepath.Join(s.base, "blocked")
	if _, _, err := s.m4Home("sample"); err == nil {
		t.Fatal("a home under a file was made")
	}
}

func TestM4RepositoryEnrollsInTheTargetHome(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	home, env, err := s.m4Home("row")
	if err != nil {
		t.Fatal(err)
	}
	var seen []command
	base := s.execute
	s.execute = func(ctx context.Context, c command) result {
		seen = append(seen, c)
		return base(ctx, c)
	}
	target := m4Target{home: home, binary: s.binary, env: env}
	repo, err := s.m4Repository(t.Context(), target, "m4-row")
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[0].program != s.git || seen[1].program != s.binary ||
		!slices.Equal(seen[1].args, []string{commandInit, flagApply}) || environmentValue(seen[1].env, "HOME") != home {
		t.Fatalf("commands = %+v", seen)
	}
	if _, err := os.Stat(filepath.Join(repo, policyFile)); err != nil {
		t.Fatalf("the repository was not enrolled: %v", err)
	}
	s.execute = func(context.Context, command) result { return result{code: 1} }
	if _, err := s.m4Repository(t.Context(), target, "m4-failed"); err == nil || !strings.Contains(err.Error(), "prepare the repository m4-failed") {
		t.Fatalf("a failed init = %v", err)
	}
	if got := s.m4Shared("repo"); got.home != s.home || got.binary != s.binary || got.repo != "repo" || !slices.Equal(got.env, s.env) {
		t.Fatalf("shared target = %+v", got)
	}
}

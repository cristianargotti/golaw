package main

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/runner"
)

func TestM2TimingFixtureReuseAndIsolation(t *testing.T) {
	t.Parallel()
	for _, name := range rung.Names() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s, observed := m2TimingFixtureSuite(t, name)
			parent := slices.Clone(s.env)
			tools := m2DatabaseDigests(t, s.m2.Bin)
			cache := filepath.Join(s.base, "sample caches", "warm")
			first := m2PreparedTiming(t, s, name, cache)
			state := m2TimingFixtureState(t, s, first)
			calls := observed.calls
			second := m2PreparedTiming(t, s, name, cache)
			if observed.calls != calls || !reflect.DeepEqual(first, second) {
				t.Fatal("repeated timing preparation executed a command or changed the invocation")
			}
			if !reflect.DeepEqual(state, m2TimingFixtureState(t, s, second)) {
				t.Fatal("repeated timing preparation changed HEAD, index or checkout bytes")
			}
			otherCache := filepath.Join(s.base, "sample caches", "cold")
			other := m2PreparedTiming(t, s, name, otherCache)
			if first.dir == other.dir {
				t.Fatal("separate cache roots reused a timing repository")
			}
			m2RequireTimingCachePaths(t, s, first, cache)
			m2RequireTimingCachePaths(t, s, other, otherCache)
			m2RequireTimingArguments(t, s, first, name)
			m2RequireTimingPreparationCounts(t, observed, name, 2)
			m2RequireTimingCopies(t, s, name, cache, first)
			if !slices.Equal(parent, s.env) || !maps.Equal(tools, m2DatabaseDigests(t, s.m2.Bin)) {
				t.Fatal("timing preparation changed parent caches or verified tool bytes")
			}
			m2RequireTimingSourceUnchanged(t, s, observed)
		})
	}
}

func m2PreparedTiming(t *testing.T, s *suite, name, cache string) command {
	t.Helper()
	c, err := s.timingM2(t.Context(), name, cache)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func m2TimingFixtureState(t *testing.T, s *suite, c command) m2State {
	t.Helper()
	state, err := s.captureM2(t.Context(), c.dir, c.env)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func m2RequireTimingCopies(t *testing.T, s *suite, name, cache string, c command) {
	t.Helper()
	want := m2CopyCommand(c)
	c.args[0], c.env[0], c.dir, c.input = "changed", "changed=value", "changed", "changed"
	got := m2PreparedTiming(t, s, name, cache)
	if !reflect.DeepEqual(got, want) {
		t.Fatal("caller changes reached the cached command")
	}
}

func m2RequireTimingCachePaths(t *testing.T, s *suite, c command, cache string) {
	t.Helper()
	for name, suffix := range map[string]string{
		"GOCACHE": "go-build", "GOMODCACHE": "go-mod", "GOLANGCI_LINT_CACHE": "golangci-lint",
	} {
		path := environmentValue(c.env, name)
		if path != filepath.Join(cache, suffix) || strings.HasPrefix(s.m2.Bin, path+string(os.PathSeparator)) {
			t.Fatalf("%s=%q does not isolate sample caches from verified tools", name, path)
		}
	}
	if environmentValue(c.env, "HOME") == s.home || c.dir == s.root {
		t.Fatal("timing fixture reused its parent home or source checkout")
	}
}

func m2RequireTimingArguments(t *testing.T, s *suite, c command, name string) {
	t.Helper()
	if c.program == s.binary || c.program == s.goTool || filepath.Base(c.args[0]) != name {
		t.Fatal("timing invocation bypasses the installed hook")
	}
	switch name {
	case rung.PreCommit:
		if len(c.args) != 1 || c.input != "" {
			t.Fatalf("pre-commit arguments=%q input=%q", c.args, c.input)
		}
	case rung.CommitMsg:
		if len(c.args) != 2 || !strings.Contains(c.args[1], " ") || c.input != "" {
			t.Fatalf("commit-msg lost its message path argument: %q", c.args)
		}
		body, err := os.ReadFile(c.args[1])
		if err != nil || string(body) != m2MessageBody {
			t.Fatalf("commit message=%q error=%v", body, err)
		}
	case rung.PrePush:
		head := strings.TrimSpace(m2TimingReadGit(t.Context(), t, s, c, "rev-parse", "HEAD"))
		want := "refs/heads/main " + head + " refs/heads/main " + runner.ZeroSHA + "\n"
		if len(c.args) != 3 || c.args[1] != "local" || c.input != want {
			t.Fatalf("pre-push arguments=%q input=%q want=%q", c.args, c.input, want)
		}
	}
}

func m2RequireTimingPreparationCounts(t *testing.T, observed *m2TimingFixture, name string, repositories int) {
	t.Helper()
	commits, builds := repositories, 0
	if name == rung.PrePush {
		commits, builds = 2*repositories, repositories
	}
	if observed.setups != repositories || observed.commits != commits || observed.builds != builds {
		t.Fatalf("preparation counts: setup=%d commit=%d build=%d; want %d %d %d",
			observed.setups, observed.commits, observed.builds, repositories, commits, builds)
	}
}

func TestM2TimingFixturePushDatabaseIsolation(t *testing.T) {
	t.Parallel()
	s, observed := m2TimingFixtureSuite(t, rung.PrePush)
	cache := filepath.Join(s.base, "push cache")
	fixture := m2PreparedTiming(t, s, rung.PrePush, cache)
	scanner := filepath.Join(s.m2.Bin, executable(m2Scanner))
	if environmentValue(fixture.env, m2RealScannerKey) != scanner ||
		!strings.HasSuffix(environmentValue(fixture.env, m2DatabaseKey), "/empty-db") {
		t.Fatal("fixture timing did not retain the verified scanner and local empty database")
	}
	proxy, proxyErr := resolveExecutable(m2Scanner, fixture.env)
	if proxyErr != nil || proxy != observed.proxy || proxy == scanner {
		t.Fatalf("fixture scanner bypasses its prepared proxy: %s %v", proxy, proxyErr)
	}
	m2RequireTimingPushTools(t, fixture.env)
	s.m2.Live = true
	live := m2PreparedTiming(t, s, rung.PrePush, cache)
	if fixture.dir == live.dir || environmentValue(live.env, m2DatabaseKey) != "" ||
		environmentValue(live.env, m2RealScannerKey) != "" {
		t.Fatal("live timing reused the fixture repository or database proxy routing")
	}
	m2RequireTimingPushTools(t, live.env)
	path, err := resolveExecutable(m2Scanner, live.env)
	if err != nil {
		t.Fatal(err)
	}
	got, readErr := os.ReadFile(path)
	want, sourceErr := os.ReadFile(scanner)
	if readErr != nil || sourceErr != nil || !bytes.Equal(got, want) {
		t.Fatalf("live scanner differs from the verified tool: %v %v", readErr, sourceErr)
	}
	if observed.setups != 2 || observed.builds != 1 {
		t.Fatalf("live timing reused setup or built a proxy: setup=%d build=%d", observed.setups, observed.builds)
	}
}

func m2RequireTimingPushTools(t *testing.T, env []string) {
	t.Helper()
	for _, name := range []string{m2Scanner, "lefthook", "go", m2GitCommand, "sh", binaryName} {
		if _, err := resolveExecutable(name, env); err != nil {
			t.Fatalf("push prerequisite %s is missing: %v", name, err)
		}
	}
	for _, name := range []string{"gitleaks", "golangci-lint"} {
		if path, err := resolveExecutable(name, env); err == nil {
			t.Fatalf("push resolves unrelated tool %s at %s", name, path)
		}
	}
}

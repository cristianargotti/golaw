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

type m2TimingFixture struct {
	calls   int
	builds  int
	setups  int
	commits int
	proxy   string
	source  map[string][]byte
}

func m2TimingFixtureSuite(t *testing.T, name string) (*suite, *m2TimingFixture) {
	t.Helper()
	s := m2TestSuite(t)
	s.env = append(s.env, "GOLANGCI_LINT_CACHE="+filepath.Join(s.base, "shared lint cache"))
	observed := &m2TimingFixture{source: make(map[string][]byte)}
	names := []string{moduleFile, m2TimingSourcePath, policyFile, "lefthook.yml"}
	for _, path := range names {
		body, err := os.ReadFile(filepath.Join(s.root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		observed.source[path] = body
	}
	script := m2TimingScript(t, s, "answer")
	s.execute = func(ctx context.Context, c command) result {
		observed.calls++
		m2ObserveTimingPreparation(ctx, t, s, observed, c, name)
		if c.program == s.git && slices.Contains(c.args, "ls-files") {
			return result{stdout: strings.Join(names, "\x00") + "\x00"}
		}
		return script.Execute(ctx, c)
	}
	return s, observed
}

func m2ObserveTimingPreparation(ctx context.Context, t *testing.T, s *suite, observed *m2TimingFixture, c command, name string) {
	t.Helper()
	if m2InstalledHook(s, c) {
		t.Fatal("timing preparation executed an installed hook")
	}
	if c.program == s.git && slices.Contains(c.args, "commit") {
		observed.commits++
		if m2Armed(ctx, t, s, c) {
			t.Fatal("timing preparation committed after setup armed hooks")
		}
	}
	if c.program == s.goTool && len(c.args) != 0 && c.args[0] == goBuild {
		observed.builds++
		if !strings.Contains(c.args[3], m2ScannerProxyDir) {
			t.Fatalf("timing preparation built a candidate or tool: %q", c.args)
		}
		observed.proxy = c.args[3]
	}
	if m2Kind(s, c) == "setup" {
		observed.setups++
		if !slices.Equal(c.args, []string{m2SetupCommand}) || environmentValue(c.env, "GOBIN") != s.m2.Bin {
			t.Fatal("timing setup does not reuse the verified shared tool directory")
		}
		m2RequireTimingStaging(ctx, t, s, observed, c, name)
		m2RequireTimingAdopter(ctx, t, s, observed, c)
	}
}

func m2RequireTimingStaging(ctx context.Context, t *testing.T, s *suite, observed *m2TimingFixture, c command, name string) {
	t.Helper()
	original := string(observed.source[m2TimingSourcePath])
	expected := original + m2TimingLine
	indexed := m2TimingReadGit(ctx, t, s, c, "show", ":"+m2TimingSourcePath)
	head := m2TimingReadGit(ctx, t, s, c, "show", "HEAD:"+m2TimingSourcePath)
	changed := strings.TrimSpace(m2TimingReadGit(ctx, t, s, c, "diff", "--cached", "--name-only"))
	if indexed != expected {
		t.Fatal("timing fixture did not stage exactly the supplied comment")
	}
	if name == "pre-push" {
		parent := m2TimingReadGit(ctx, t, s, c, "show", "HEAD^:"+m2TimingSourcePath)
		if head != expected || parent != original || changed != "" {
			t.Fatal("push comment was not committed before setup")
		}
	} else if head != original || changed != m2TimingSourcePath {
		t.Fatal("timing staged source differs from its baseline")
	}
	checkout, err := os.ReadFile(filepath.Join(c.dir, filepath.FromSlash(m2TimingSourcePath)))
	if err != nil || string(checkout) != expected {
		t.Fatalf("timing checkout source=%q error=%v", checkout, err)
	}
}

func m2RequireTimingAdopter(ctx context.Context, t *testing.T, s *suite, observed *m2TimingFixture, c command) {
	t.Helper()
	policy := m2TimingReadGit(ctx, t, s, c, "show", "HEAD:"+policyFile)
	scoped := *s
	scoped.env = c.env
	m2RequirePinRecord(t, &scoped, c.dir, m2CandidateVersion, policy)
	body, err := os.ReadFile(filepath.Join(c.dir, "lefthook.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(body, observed.source["lefthook.yml"]) {
		t.Fatal("timing retained the kit's development hook configuration")
	}
	assets := m2Render(t, c.dir)
	for i := range assets {
		if assets[i].Path == "lefthook.yml" {
			if !bytes.Equal(assets[i].Data, body) {
				t.Fatal("timing hook configuration differs from the adopter asset")
			}
			return
		}
	}
	t.Fatal("rendered adopter hook configuration is absent")
}

func m2TimingReadGit(ctx context.Context, t *testing.T, s *suite, c command, args ...string) string {
	t.Helper()
	got := m2FixtureExecutor(t, s)(ctx, command{
		program: s.git, dir: c.dir, env: c.env,
		args: append([]string{"-c", "core.quotePath=false"}, args...),
	})
	if err := success(got); err != nil {
		t.Fatal(err)
	}
	return got.stdout
}

func m2RequireTimingSourceUnchanged(t *testing.T, s *suite, observed *m2TimingFixture) {
	t.Helper()
	for path, original := range observed.source {
		body, err := os.ReadFile(filepath.Join(s.root, filepath.FromSlash(path)))
		if err != nil || !bytes.Equal(body, original) {
			t.Fatalf("timing changed original source %s: %v", path, err)
		}
	}
}

package policy_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestCompileFull(t *testing.T) {
	t.Parallel()
	f := decode(t, fullPolicy)
	p, err := policy.Compile(f, now(), schemas())
	if err != nil {
		t.Fatal(err)
	}
	switch {
	case p.Digest != policy.Digest(f), p.KitVersion != "0.1.0", p.Profile != "acme", p.Language != "en-US":
		t.Errorf("head = %+v", p)
	case !slices.Equal(p.Capabilities, []string{"cli", "go"}):
		t.Errorf("capabilities = %v", p.Capabilities)
	case p.Sizes != (model.SizeFloors{FileLines: 250, FunctionLines: 30}):
		t.Errorf("sizes = %+v", p.Sizes)
	case !slices.Equal(p.Terms.Tools, []string{"acme-tool", "other"}), !slices.Equal(p.Terms.AllowPaths, []string{"agents/", "docs/"}):
		t.Errorf("terms = %+v", p.Terms)
	case p.Shell != (model.ShellPolicy{DenyPush: true}):
		t.Errorf("shell = %+v", p.Shell)
	case p.Enrollment.State != model.EnrollmentNone:
		t.Errorf("enrollment = %+v", p.Enrollment)
	}
	if len(p.Exceptions) != 1 || p.Exceptions[0] != (model.Exception{Check: "goast.secret-fields", Path: "internal/auth/session.go", Record: "docs/adr/0009-session-token.md"}) {
		t.Errorf("the expired exception must be dropped: %+v", p.Exceptions)
	}
	if record, ok := p.Excepted("goast.secret-fields", "internal/auth/session.go"); !ok || record != "docs/adr/0009-session-token.md" {
		t.Error("Excepted must find the unexpired exception")
	}
}

func TestCompileOptions(t *testing.T) {
	t.Parallel()
	p, err := policy.Compile(decode(t, fullPolicy), now(), schemas())
	if err != nil {
		t.Fatal(err)
	}
	coverage := p.OptionsFor(policy.CoverageCheck)
	if total, _ := coverage.Int("total"); total != 85 {
		t.Errorf("coverage options = %+v", coverage)
	}
	if exemptions, _ := coverage.Strings("file_exemptions"); !slices.Equal(exemptions, []string{"internal/legacy/old.go"}) {
		t.Errorf("file_exemptions = %v", exemptions)
	}
	if entries, _ := p.OptionsFor(policy.LintCheck).Strings("allowed_exclusions"); !slices.Equal(entries, []string{`_test\.go:funlen`, `_test\.go:mnd`}) {
		t.Errorf("allowed_exclusions = %v", entries)
	}
	terms := p.OptionsFor("text.tool-terms")
	limit, _ := terms.Int("limit")
	strict, _ := terms.Bool("strict")
	names, _ := terms.Strings("names")
	if limit != 3 || !strict || !slices.Equal(names, []string{"a", "b"}) {
		t.Errorf("per-check options = %+v", terms)
	}
}

func TestCompileMinimal(t *testing.T) {
	t.Parallel()
	p, err := policy.Compile(decode(t, minimalPolicy), now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	core := model.CoreDefaults()
	if p.Language != core.Language || p.Sizes != core.Sizes || len(p.Capabilities) != 0 || len(p.Options) != 0 || len(p.Exceptions) != 0 {
		t.Errorf("minimal = %+v", p)
	}
	if p.Terms.Tools == nil || p.Terms.AllowPaths == nil || p.Capabilities == nil {
		t.Error("lists are empty, never nil")
	}
	if _, err := policy.Compile(decode(t, "golaw: 1\n"), now(), nil); !errors.Is(err, model.ErrPolicy) {
		t.Errorf("Compile validates: %v", err)
	}
}

func TestExpired(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 12, 31, 23, 59, 0, 0, time.UTC)
	if policy.Expired("2026-12-31", day) || !policy.Expired("2026-12-31", day.Add(time.Minute)) || !policy.Expired("soon", day) {
		t.Error("an entry is in force through the whole of its last day")
	}
}

func TestDigest(t *testing.T) {
	t.Parallel()
	f := decode(t, fullPolicy)
	same := decode(t, strings.Replace(fullPolicy, "capabilities: [go, cli]", "capabilities: [cli, go]", 1))
	ladder := decode(t, strings.Replace(fullPolicy, "pre_push: false", "pre_push: true", 1))
	managed := decode(t, strings.Replace(fullPolicy, strings.Repeat("ab", 32), strings.Repeat("cd", 32), 1))
	floor := decode(t, strings.Replace(fullPolicy, "file_lines: 250", "file_lines: 200", 1))
	exception := decode(t, strings.Replace(fullPolicy, "until: 2020-01-01", "until: 2030-01-01", 1))
	d := policy.Digest(f)
	if len(d) != 64 || policy.Digest(same) != d || policy.Digest(ladder) != d || policy.Digest(managed) != d {
		t.Error("order, the ladder and the managed hashes never change the digest")
	}
	if policy.Digest(floor) == d || policy.Digest(exception) == d {
		t.Error("a floor and an exception change the digest")
	}
	if policy.Digest(decode(t, minimalPolicy)) == policy.Digest(decode(t, minimalPolicy+"checks: {terms: {tools: []}}\n")) {
		t.Error("an explicit empty block is a different policy text but the same floors")
	}
}

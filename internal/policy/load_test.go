package policy_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/source"
)

func TestLoadCoreAndUnenrolled(t *testing.T) {
	t.Parallel()
	root := initRepo(t)
	write(t, root, "a.txt", "a\n")
	commitAll(t, root, "one")
	l, err := policy.Load(t.Context(), input(root, dirs(t)))
	if err != nil || l.Origin != policy.OriginCore || l.Policy.Enrollment.State != model.EnrollmentNone || l.Policy.Sizes.FileLines != 300 {
		t.Errorf("core defaults = %+v, %v", l, err)
	}
	write(t, root, "golaw.yml", fullPolicy)
	commitAll(t, root, "policy")
	write(t, dirs(t).Config, "x", "")
	d := dirs(t)
	write(t, d.Config, "profiles/acme/profile.yml", acmeStamp)
	l, err = policy.Load(t.Context(), input(root, d))
	if err != nil || l.Origin != policy.OriginHead || l.Policy.Enrollment.State != model.EnrollmentNone || l.Policy.Sizes.FileLines != 250 || l.Stamp == nil {
		t.Errorf("unenrolled with a policy at HEAD = %+v, %v", l, err)
	}
	if _, err := policy.Load(t.Context(), input(t.TempDir(), d)); !errors.Is(err, source.ErrNotRepository) {
		t.Errorf("scan outside git: %v", err)
	}
}

func TestLoadEnrolled(t *testing.T) {
	t.Parallel()
	root, d := enrolled(t, fullPolicy)
	write(t, root, "docs/x.md", "x\n")
	l, err := policy.Load(t.Context(), input(filepath.Join(root, "docs"), d))
	if err != nil {
		t.Fatal(err)
	}
	if l.Origin != policy.OriginHead || l.Policy.Enrollment.State != model.EnrollmentEnrolled || l.Record == nil || l.Policy.Enrollment.PolicyDigest != l.Policy.Digest {
		t.Errorf("enrolled = %+v", l)
	}
	write(t, root, "golaw.yml", "golaw: [\n")
	if again, err := policy.Load(t.Context(), input(root, d)); err != nil || again.Policy.Digest != l.Policy.Digest {
		t.Errorf("the working tree is never read: %v", err)
	}
	in := input(root, d)
	in.Version = "0.2.0"
	if _, err := policy.Load(t.Context(), in); !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), "pins kit version 0.1.0 and this binary is 0.2.0") {
		t.Errorf("A10 refuses a newer binary: %v", err)
	}
	in.Version = ""
	if _, err := policy.Load(t.Context(), in); err != nil {
		t.Errorf("a development build skips the pin: %v", err)
	}
}

// tampering is one policy tampering case of section 10.5 and the words
// Load must answer it with.
type tampering struct {
	tamper func(t *testing.T, root string, d policy.Dirs)
	want   string
}

func tamperings() map[string]tampering {
	return map[string]tampering{
		"config deletion": {func(t *testing.T, root string, _ policy.Dirs) {
			t.Helper()
			gitRun(t, root, "rm", "-q", "golaw.yml")
			gitRun(t, root, "commit", "-q", "-m", "drop")
		}, "missing-policy"},
		"self-issued exception": {func(t *testing.T, root string, _ policy.Dirs) {
			t.Helper()
			write(t, root, "golaw.yml", strings.Replace(fullPolicy, "path: docs/old.md", "path: docs/new.md", 1))
			commitAll(t, root, "exception")
		}, "mismatch"},
		"weakened floor": {func(t *testing.T, root string, _ policy.Dirs) {
			t.Helper()
			write(t, root, "golaw.yml", strings.Replace(fullPolicy, "file_lines: 250", "file_lines: 300", 1))
			commitAll(t, root, "weaken")
		}, "mismatch"},
		"profile substitution": {func(t *testing.T, root string, d policy.Dirs) {
			t.Helper()
			other := strings.Repeat("9", 64)
			write(t, root, "golaw.yml", strings.Replace(fullPolicy, acmeDigest, other, 1))
			write(t, d.Config, "profiles/acme/profile.yml", strings.Replace(acmeStamp, acmeDigest, other, 1))
			commitAll(t, root, "swap")
		}, "mismatch"},
		"bundle removed": {func(t *testing.T, _ string, d policy.Dirs) {
			t.Helper()
			if err := os.RemoveAll(filepath.Join(d.Config, "profiles")); err != nil {
				t.Fatal(err)
			}
		}, "not installed"},
	}
}

func TestLoadTampering(t *testing.T) {
	t.Parallel()
	for name, c := range tamperings() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, d := enrolled(t, fullPolicy)
			c.tamper(t, root, d)
			_, err := policy.Load(t.Context(), input(root, d))
			if err == nil || !strings.Contains(err.Error(), c.want) || (!errors.Is(err, model.ErrEnrollment) && !errors.Is(err, model.ErrPolicy)) {
				t.Errorf("Load = %v, want an error mentioning %q", err, c.want)
			}
		})
	}
}

func TestLoadBootstrap(t *testing.T) {
	t.Parallel()
	root := initRepo(t)
	d := dirs(t)
	write(t, root, "golaw.yml", minimalPolicy)
	rec := policy.Record{Root: root, PolicyDigest: policy.Digest(decode(t, minimalPolicy)), KitVersion: "0.1.0", Bootstrap: minimalPolicy, WrittenBy: policy.WrittenByInit, WrittenAt: now()}
	writeRecord(t, d.State, rec)
	l, err := policy.Load(t.Context(), input(root, d))
	if err != nil || l.Origin != policy.OriginBootstrap || l.Policy.Enrollment.State != model.EnrollmentEnrolled {
		t.Errorf("unborn branch under the bootstrap record = %+v, %v", l, err)
	}
	rec.Bootstrap = ""
	writeRecord(t, d.State, rec)
	_, err = policy.Load(t.Context(), input(root, d))
	if !errors.Is(err, model.ErrEnrollment) {
		t.Errorf("a record without a bootstrap on an unborn branch: %v", err)
	}
	write(t, root, "golaw.yml", "golaw: 2\n")
	rec.Bootstrap = "golaw: 2\n"
	writeRecord(t, d.State, rec)
	_, err = policy.Load(t.Context(), input(root, d))
	if !errors.Is(err, model.ErrPolicy) {
		t.Errorf("an invalid bootstrap policy: %v", err)
	}
}

func TestLoadDiscoveryFailsWhenEnrolled(t *testing.T) {
	t.Parallel()
	root, d := enrolled(t, minimalPolicy)
	if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
	_, err := policy.Load(t.Context(), input(root, d))
	if !errors.Is(err, model.ErrEnrollment) || !errors.Is(err, source.ErrNotRepository) {
		t.Errorf("enrolled repository whose discovery fails: %v", err)
	}
}

func TestProposed(t *testing.T) {
	t.Parallel()
	root, d := enrolled(t, fullPolicy)
	l, err := policy.Load(t.Context(), input(root, d))
	if err != nil {
		t.Fatal(err)
	}
	_, found, err := policy.Proposed(t.Context(), l, schemas())
	if err != nil || found {
		t.Errorf("the committed file is the policy in force: %v, %v", found, err)
	}
	write(t, root, "golaw.yml", strings.Replace(fullPolicy, "file_lines: 250", "file_lines: 300", 1))
	gitRun(t, root, "add", "golaw.yml")
	_, found, err = policy.Proposed(t.Context(), l, schemas())
	if !errors.Is(err, model.ErrPolicy) || found {
		t.Errorf("a staged weakening is refused: %v, %v", err, found)
	}
	write(t, root, "golaw.yml", strings.Replace(fullPolicy, "file_lines: 250", "file_lines: 200", 1))
	gitRun(t, root, "add", "golaw.yml")
	next, found, err := policy.Proposed(t.Context(), l, schemas())
	if err != nil || !found || *next.Checks.Sizes.FileLines != 200 {
		t.Errorf("a staged tightening = %+v, %v, %v", next, found, err)
	}
	write(t, root, "golaw.yml", "golaw: [\n")
	gitRun(t, root, "add", "golaw.yml")
	if _, _, err := policy.Proposed(t.Context(), l, schemas()); !errors.Is(err, model.ErrPolicy) {
		t.Errorf("a malformed staged file: %v", err)
	}
}

func TestProposedValidatesBeforeTheDigest(t *testing.T) {
	t.Parallel()
	root, d := enrolled(t, fullPolicy)
	l, err := policy.Load(t.Context(), input(root, d))
	if err != nil {
		t.Fatal(err)
	}
	staged := map[string]string{
		"a value the encoder refuses":         strings.Replace(fullPolicy, "limit: 3", "limit: .nan", 1),
		"an infinite value":                   strings.Replace(fullPolicy, "limit: 3", "limit: .inf", 1),
		"the policy in force, invalid around": strings.Replace(fullPolicy, abHash, "abc", 1),
	}
	for name, text := range staged {
		write(t, root, "golaw.yml", text)
		gitRun(t, root, "add", "golaw.yml")
		if _, found, err := policy.Proposed(t.Context(), l, schemas()); !errors.Is(err, model.ErrPolicy) || found {
			t.Errorf("%s: %v, %v, want ErrPolicy", name, found, err)
		}
	}
}

func TestProposedFromCore(t *testing.T) {
	t.Parallel()
	d := dirs(t)
	core := initRepo(t)
	write(t, core, "a.txt", "a\n")
	commitAll(t, core, "one")
	base, err := policy.Load(t.Context(), input(core, d))
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := policy.Proposed(t.Context(), base, schemas()); err != nil || found {
		t.Errorf("nothing staged: %v, %v", found, err)
	}
	write(t, core, "golaw.yml", fullPolicy)
	gitRun(t, core, "add", "golaw.yml")
	if _, found, err := policy.Proposed(t.Context(), base, schemas()); err != nil || !found {
		t.Errorf("the first policy only needs to be valid: %v, %v", found, err)
	}
	write(t, core, "golaw.yml", "golaw: 2\n")
	gitRun(t, core, "add", "golaw.yml")
	if _, _, err := policy.Proposed(t.Context(), base, schemas()); !errors.Is(err, model.ErrPolicy) {
		t.Errorf("an invalid first policy: %v", err)
	}
	write(t, core, "golaw.yml", minimalPolicy+"checks: {text.tool-terms: {limit: 1000}}\n")
	gitRun(t, core, "add", "golaw.yml")
	if _, found, err := policy.Proposed(t.Context(), base, schemas()); !errors.Is(err, model.ErrPolicy) || found {
		t.Errorf("a first policy whose option weakens its core default: %v, %v", found, err)
	}
}

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/policy"
)

// capabilityRun runs one command of the bound app and answers its stdout,
// failing the test on another exit code.
func capabilityRun(t *testing.T, ta testApp, code int, args ...string) string {
	t.Helper()
	ta.stdout.Reset()
	ta.stderr.Reset()
	if got := ta.run(t.Context(), args); got != code {
		t.Fatalf("%v: exit = %d, want %d, stdout = %q, stderr = %q", args, got, code, ta.stdout.String(), ta.stderr.String())
	}
	return ta.stdout.String()
}

// adopted is a Go module init --apply adopted, committed when commit is true.
func adopted(t *testing.T, ta testApp, commit bool) *gittest.Repo {
	t.Helper()
	repo := gittest.New(t)
	repo.Write("go.mod", "module example.test/service\n\ngo 1.27\n")
	capabilityRun(t, ta, exitClean, "init", "--apply", repo.Root)
	if commit {
		repo.Git("add", "-A")
		repo.Commit("build: adopt golaw", nil)
	}
	return repo
}

// enrolledDigest is the policy digest the enrollment record of root holds.
func enrolledDigest(t *testing.T, ta testApp, root string) string {
	t.Helper()
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	record, found, err := policy.ReadRecord(dirs.State, root)
	if err != nil || !found {
		t.Fatalf("record = %+v, %v, %v", record, found, err)
	}
	return record.PolicyDigest
}

func currentPolicy(t *testing.T, root string) []byte {
	t.Helper()
	data, err := bootstrap.Current(root)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestInitGrowsTheCapabilitiesOfAReviewedPolicy(t *testing.T) {
	t.Parallel()
	ta := boundApp(t)
	repo := adopted(t, ta, true)
	before := currentPolicy(t, repo.Root)
	preview := capabilityRun(t, ta, exitClean, "init", "--capabilities", "hexagonal", repo.Root)
	for _, want := range []string{"-capabilities: [go]\n", "+capabilities: [go, hexagonal]\n", "init: nothing written, run again with --apply\n"} {
		if !strings.Contains(preview, want) {
			t.Errorf("preview lacks %q:\n%s", want, preview)
		}
	}
	if !bytes.Equal(currentPolicy(t, repo.Root), before) {
		t.Fatal("the preview wrote golaw.yml")
	}
	applied := capabilityRun(t, ta, exitClean, "init", "--apply", "--capabilities", "hexagonal", repo.Root)
	grown := currentPolicy(t, repo.Root)
	if !strings.Contains(string(grown), "\ncapabilities: [go, hexagonal]\n") || !strings.HasSuffix(applied, enrollAfterCommitLine("pt-BR")+"\n") {
		t.Fatalf("applied = %q, policy = %q", applied, grown)
	}
	if got := enrolledDigest(t, ta, repo.Root); got != digestOf(before) {
		t.Fatalf("the record moved before the commit: %s", got)
	}
	capabilityRun(t, ta, exitClean, "scan", repo.Root)
	repo.Git("add", "-A")
	repo.Commit("build: enable hexagonal", nil)
	if out := capabilityRun(t, ta, exitClean, "init", "--apply", repo.Root); !strings.Contains(out, "enrolled: ") {
		t.Fatalf("second apply = %q", out)
	}
	if got := enrolledDigest(t, ta, repo.Root); got != digestOf(grown) {
		t.Fatalf("the record does not hold the grown policy: %s", got)
	}
	capabilityRun(t, ta, exitClean, "scan", repo.Root)
	kept := capabilityRun(t, ta, exitClean, "init", "--capabilities", "db", repo.Root)
	if !strings.Contains(kept, "+capabilities: [db, go, hexagonal]\n") {
		t.Fatalf("a capability the flag leaves out was not kept:\n%s", kept)
	}
}

func TestInitEnrollsAGrownPolicyAtOnceBeforeTheFirstCommit(t *testing.T) {
	t.Parallel()
	ta := boundApp(t)
	repo := adopted(t, ta, false)
	applied := capabilityRun(t, ta, exitClean, "init", "--apply", "--capabilities", "cli", repo.Root)
	grown := currentPolicy(t, repo.Root)
	if !strings.Contains(string(grown), "\ncapabilities: [cli, go]\n") || !strings.HasSuffix(applied, commitAsked) {
		t.Fatalf("applied = %q, policy = %q", applied, grown)
	}
	if got := enrolledDigest(t, ta, repo.Root); got != digestOf(grown) {
		t.Fatalf("the record does not hold the grown bootstrap policy: %s", got)
	}
}

func TestInitCapabilitiesRefusals(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		policy string
		args   []string
		want   string
	}{
		"beside --update": {"", []string{"--update", "--capabilities", "api"}, "--update keeps the capabilities of golaw.yml; run golaw init --capabilities first"},
		"unknown":         {"", []string{"--capabilities", "web"}, `unknown capability "web"`},
		"a shape alone":   {"golaw: 1\nkit: {version: " + testVersion + "}\ncapabilities: [image]\n", []string{"--capabilities", "api"}, "capability api needs go: add go to the capabilities"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ta := boundApp(t)
			repo := adopted(t, ta, false)
			if tc.policy != "" {
				if err := os.WriteFile(filepath.Join(repo.Root, bootstrap.PolicyFile), []byte(tc.policy), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			before := currentPolicy(t, repo.Root)
			capabilityRun(t, ta, exitFailure, append(append([]string{"init"}, tc.args...), repo.Root)...)
			if !strings.Contains(ta.stderr.String(), tc.want) || !bytes.Equal(currentPolicy(t, repo.Root), before) {
				t.Fatalf("stderr = %q", ta.stderr.String())
			}
		})
	}
}

func TestInitKeepsCapabilityEnrollmentUntilCommitOnEveryApply(t *testing.T) {
	t.Parallel()
	for name, flags := range map[string][]string{
		"no capability flag": nil,
		"same capability":    {"--capabilities", "hexagonal"},
		"another capability": {"--capabilities", "cli"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ta := boundApp(t)
			repo := adopted(t, ta, true)
			held := enrolledDigest(t, ta, repo.Root)
			capabilityRun(t, ta, exitClean, "init", "--apply", "--capabilities", "hexagonal", repo.Root)
			args := append(append([]string{"init", "--apply"}, flags...), repo.Root)
			out := capabilityRun(t, ta, exitClean, args...)
			if got := enrolledDigest(t, ta, repo.Root); got != held {
				t.Errorf("record moved before the commit: got %s, want %s", got, held)
			}
			if !strings.HasSuffix(out, enrollAfterCommitLine("pt-BR")+"\n") {
				t.Errorf("apply lacks the capability commit reminder: %q", out)
			}
			capabilityRun(t, ta, exitClean, "scan", repo.Root)
		})
	}
}

func TestInitDefersCapabilityGrowthAfterAnUnenrolledCommit(t *testing.T) {
	t.Parallel()
	ta := boundApp(t)
	repo := adopted(t, ta, true)
	held := digestOf(currentPolicy(t, repo.Root))
	capabilityRun(t, ta, exitClean, "init", "--apply", "--capabilities", "hexagonal", repo.Root)
	repo.Commit("build: enable hexagonal", map[string]string{
		bootstrap.PolicyFile: string(currentPolicy(t, repo.Root)),
	})
	out := capabilityRun(t, ta, exitClean, "init", "--apply", "--capabilities", "db", repo.Root)
	grown := currentPolicy(t, repo.Root)
	if !strings.Contains(string(grown), "\ncapabilities: [db, go, hexagonal]\n") {
		t.Fatalf("grown policy lacks the added capabilities: %q", grown)
	}
	if got := enrolledDigest(t, ta, repo.Root); got != held {
		t.Errorf("record advanced ahead of HEAD after another growth: got %s, want %s", got, held)
	}
	if !strings.HasSuffix(out, enrollAfterCommitLine("pt-BR")+"\n") {
		t.Error("another growth lacks the capability commit reminder")
	}
	repo.Commit("build: enable database", map[string]string{bootstrap.PolicyFile: string(grown)})
	capabilityRun(t, ta, exitClean, "init", "--apply", repo.Root)
	if got, want := enrolledDigest(t, ta, repo.Root), digestOf(grown); got != want {
		t.Fatalf("committed growth was not enrolled: got %s, want %s", got, want)
	}
	capabilityRun(t, ta, exitClean, "scan", repo.Root)
}

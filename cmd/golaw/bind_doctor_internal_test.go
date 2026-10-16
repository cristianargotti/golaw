package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/doctor"
)

func TestDoctorReportCollectsTheEvidenceOfARepository(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	root := repository(t, true)
	report, err := b.doctorReport(t.Context(), agentUnderTest(), root, scopeUser, now())
	if err != nil || report.Schema != 1 || len(report.Lines) == 0 {
		t.Fatalf("report = %+v, %v", report, err)
	}
	ids := map[string]bool{}
	for _, line := range report.Lines {
		ids[line.ID] = true
	}
	for _, want := range []string{"doctor.artifacts", "doctor.enrollment", "doctor.pin"} {
		if !ids[want] {
			t.Errorf("line %s is missing from %v", want, ids)
		}
	}
	if last := report.Lines[len(report.Lines)-1]; last.ID != "doctor.dispatch" || last.Code != "dispatch-unobserved" {
		t.Errorf("a repository no hook process ran in = %+v", last)
	}
}

// observedRepository enrolls a repository, puts a binary on the observed
// path and lets a hook process record its environment, as doctor expects.
func observedRepository(t *testing.T, b binding, home string) string {
	t.Helper()
	root := repository(t, true)
	data := renderedPolicy(t)
	p, err := b.compile(data, now())
	if err != nil {
		t.Fatal(err)
	}
	if err = b.enroll(root, data, p, now()); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(home, "bin", "golaw")
	if err = os.MkdirAll(filepath.Dir(binary), 0o750); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(binary, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	h := hookBindingFor(t, b)
	if h.hookDigest, err = hookDigest(agentUnderTest()); err != nil {
		t.Fatal(err)
	}
	if _, err = h.enroll(t.Context(), root, root); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestDoctorReportUsesTheObservationOfTheHook(t *testing.T) {
	t.Parallel()
	b, home := testBinding(t)
	root := observedRepository(t, b, home)
	report, err := b.doctorReport(t.Context(), agentUnderTest(), root, scopeUser, now())
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range report.Lines {
		if line.ID == "doctor.binary" && line.State == "unknown" {
			t.Fatalf("the observation of the hook was not used: %+v", line)
		}
	}
	last := report.Lines[len(report.Lines)-1]
	if last.ID != "doctor.dispatch" || last.State != "ok" || !strings.Contains(last.Detail, now().Format(time.RFC3339)+" on linux") {
		t.Fatalf("the dispatch of the hook = %+v", last)
	}
	b.exe = func() (string, error) { return "", errBoom }
	if _, err = b.doctorReport(t.Context(), agentUnderTest(), root, scopeUser, now()); err == nil {
		t.Fatal("an unresolvable binary was reported")
	}
	b.env = func(string) string { return "" }
	if _, err = b.doctorReport(t.Context(), agentUnderTest(), root, scopeUser, now()); err == nil {
		t.Fatal("an unresolvable state directory was reported")
	}
}

func TestDoctorReportsAPolicyGoneFromTheCheckout(t *testing.T) {
	t.Parallel()
	b, home := testBinding(t)
	root := observedRepository(t, b, home)
	enrollment := func() doctor.Line {
		t.Helper()
		report, err := b.doctorReport(t.Context(), agentUnderTest(), root, scopeUser, now())
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range report.Lines {
			if line.ID == "doctor.enrollment" {
				return line
			}
		}
		t.Fatalf("no enrollment line in %+v", report)
		return doctor.Line{}
	}
	if line := enrollment(); line.State != "ok" || line.Code != "enrolled" {
		t.Fatalf("the unchanged checkout = %+v", line)
	}
	if err := os.Remove(filepath.Join(root, "golaw.yml")); err != nil {
		t.Fatal(err)
	}
	if line := enrollment(); line.State != "red" || !strings.Contains(line.Detail, "missing-policy") {
		t.Fatalf("the checkout without the file = %+v", line)
	}
}

func TestPolicyFactsFallBackToTheFileAtHead(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	root := repository(t, true)
	digest, profile, pin := b.policyFacts(t.Context(), root, now())
	if digest == "" || profile != "" || pin != testVersion {
		t.Fatalf("facts = %q, %q, %q", digest, profile, pin)
	}
	b.info.Version = "1.0.0"
	fallback, _, pinned := b.policyFacts(t.Context(), root, now())
	if fallback != digest || pinned != testVersion {
		t.Fatalf("fallback = %q, %q", fallback, pinned)
	}
	bare := t.TempDir()
	if digest, _, _ := b.policyFacts(t.Context(), bare, now()); digest != "" {
		t.Fatalf("a directory outside git has a digest %q", digest)
	}
	if err := os.WriteFile(filepath.Join(root, "golaw.yml"), []byte("golaw: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "build: break the policy")
	if digest, _, _ := b.policyFacts(t.Context(), root, now()); digest != "" {
		t.Fatalf("an undecodable file has a digest %q", digest)
	}
}

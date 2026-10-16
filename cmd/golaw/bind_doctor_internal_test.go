package main

import (
	"os"
	"path/filepath"
	"testing"
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
	if _, err = h.enroll(t.Context(), root); err != nil {
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
	b.exe = func() (string, error) { return "", errBoom }
	if _, err = b.doctorReport(t.Context(), agentUnderTest(), root, scopeUser, now()); err == nil {
		t.Fatal("an unresolvable binary was reported")
	}
	b.env = func(string) string { return "" }
	if _, err = b.doctorReport(t.Context(), agentUnderTest(), root, scopeUser, now()); err == nil {
		t.Fatal("an unresolvable state directory was reported")
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

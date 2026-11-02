package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// mapApp is an app whose seams reach a real repository of the growth
// fixture under the policy the test hands it.
func mapApp(t *testing.T, pol *model.EffectivePolicy) (testApp, string) {
	t.Helper()
	repo := gentest.Module(t, gentest.Contracts())
	a := newTestApp(t)
	a.load = func(context.Context, string, time.Time) (*model.EffectivePolicy, error) { return pol, nil }
	b, _ := testBinding(t)
	a.bind = func() binding { return b }
	return a, repo.Root
}

func runMap(t *testing.T, a testApp, args ...string) error {
	t.Helper()
	cmd := a.mapCommand()
	cmd.SetArgs(args)
	cmd.SetOut(a.stdout)
	return cmd.ExecuteContext(t.Context()) //nolint:wrapcheck // the test reads the command's own error
}

func TestMapPrintsTheRepositoryWithinTheBudget(t *testing.T) {
	t.Parallel()
	a, root := mapApp(t, proof.Policy())
	if err := runMap(t, a, root); err != nil {
		t.Fatal(err)
	}
	out := a.stdout.String()
	if !strings.HasPrefix(out, "golaw map layout: port internal/application/{area}") || !strings.Contains(out, "\nport growth.Ledger internal/application/growth/ports.go: methods ReadPlan, ArchivePlan\n") {
		t.Fatalf("out =\n%s", out)
	}
	a.stdout.Reset()
	if err := runMap(t, a, "--json", root); err != nil || !strings.HasPrefix(a.stdout.String(), "{") {
		t.Fatalf("json = %q, %v", a.stdout.String(), err)
	}
}

func TestMapRefusesOutsideAnEnrolledRepositoryAndABadBudget(t *testing.T) {
	t.Parallel()
	unenrolled := *proof.Policy()
	unenrolled.Enrollment = model.Enrollment{State: model.EnrollmentNone}
	a, root := mapApp(t, &unenrolled)
	err := runMap(t, a, root)
	if !errors.Is(err, model.ErrUsage) || a.exitCode(err) != exitFailure || !strings.Contains(err.Error(), "run golaw init") {
		t.Fatalf("unenrolled: %v", err)
	}
	b, root := mapApp(t, proof.Policy())
	if err := runMap(t, b, "--budget", "0", root); !errors.Is(err, model.ErrUsage) {
		t.Fatalf("budget 0: %v", err)
	}
}

func TestReadManifestTellsAnAbsentManifestFromABrokenOne(t *testing.T) {
	t.Parallel()
	if m, err := readManifest(proof.NewTree(map[string][]byte{"go.mod": []byte("module x\n")})); m != nil || err != nil {
		t.Fatalf("absent = %v, %v", m, err)
	}
	broken := proof.NewTree(map[string][]byte{manifest.Path: []byte("golaw-gen: 9\n")})
	if _, err := readManifest(broken); !errors.Is(err, manifest.ErrManifest) {
		t.Fatalf("broken = %v", err)
	}
}

func TestRepositoryMapReadsTheSnapshotOfTheRoot(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	repo := gentest.Module(t, gentest.Contracts())
	report, err := b.repositoryMap(t.Context(), repo.Root, proof.Policy(), "growth")
	if err != nil || len(report.Ports) != 2 {
		t.Fatalf("report = %+v, %v", report, err)
	}
	var buf bytes.Buffer
	if err := writeLines(&buf, []string{report.Text(10)}); err != nil || !strings.Contains(buf.String(), " more ports: golaw map --area growth") {
		t.Fatalf("a small budget = %q, %v", buf.String(), err)
	}
}

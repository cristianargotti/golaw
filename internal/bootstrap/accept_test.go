package bootstrap_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
)

// acceptPlan plans a repository born with its own lint configuration and
// the Makefile given, against a strict asset, the include line and golaw.mk.
func acceptPlan(t *testing.T, makefile string) (string, bootstrap.TemplatePlan) {
	t.Helper()
	root := t.TempDir()
	writeBefore(t, filepath.Join(root, ".golangci.yml"), "version: 2\n")
	writeBefore(t, filepath.Join(root, "Makefile"), makefile)
	plan, err := bootstrap.PlanTemplates(root, []assets.Rendered{
		templateFile(".golangci.yml", "version: \"2\"\n", assets.OwnedByKit),
		templateFile("Makefile", "include golaw.mk\n", assets.OwnedMerged),
		templateFile("golaw.mk", "lint:\n\tgolangci-lint run\n", assets.OwnedByKit),
	})
	if err != nil {
		t.Fatal(err)
	}
	return root, plan
}

func TestTemplatePlanAcceptMovesOneProposalIntoTheWrites(t *testing.T) {
	t.Parallel()
	root, plan := acceptPlan(t, "all:\n\techo\n")
	if err := plan.Accept([]string{".golangci.yml"}); err != nil {
		t.Fatal(err)
	}
	record := policy.ManagedFile{Path: ".golangci.yml", SHA256: install.Digest([]byte("version: \"2\"\n"))}
	if !slices.Contains(plan.Managed, record) ||
		slices.ContainsFunc(plan.Proposed, func(c install.Change) bool { return c.Path == ".golangci.yml" }) {
		t.Fatalf("accepted kit file: managed %+v, proposed %+v", plan.Managed, plan.Proposed)
	}
	if err := plan.Accept([]string{"Makefile"}); err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(plan.Managed, func(m policy.ManagedFile) bool { return m.Path == "Makefile" }) {
		t.Fatal("a merged file joined the managed list")
	}
	if err := bootstrap.ApplyTemplates(root, plan, io.Discard); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{".golangci.yml": "version: \"2\"\n", "Makefile": "all:\n\techo\ninclude golaw.mk\n"} {
		got, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(got) != want {
			t.Fatalf("%s = %q, %v", name, got, err)
		}
	}
}

func TestTemplatePlanAcceptRefusesWhatItCannotWrite(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, makefile, accept, want string }{
		{"red not a proposal", "all:\n\techo\n", "go.sum", "the proposals of this run are: .golangci.yml, Makefile"},
		{"red collision", "lint:\n\techo\n", "Makefile", "lint (Makefile:1)"},
		{"red go.mod", "all:\n\techo\n", "go.mod", "go get"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, plan := acceptPlan(t, tc.makefile)
			err := plan.Accept([]string{tc.accept})
			if !errors.Is(err, bootstrap.ErrAccept) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Accept(%s) = %v, want %q", tc.accept, err, tc.want)
			}
		})
	}
}

func TestTemplatePlanAcceptReplacesOnlyTheChosenLocalBaseline(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	baseline := install.Digest([]byte("recorded\n"))
	config := "golaw: 1\nkit: {version: 0.1.0}\nmanaged:\n" +
		"  - {path: a.txt, sha256: " + baseline + "}\n" +
		"  - {path: z.txt, sha256: " + baseline + "}\n"
	writeBefore(t, filepath.Join(root, bootstrap.PolicyFile), config)
	writeBefore(t, filepath.Join(root, "a.txt"), "local\n")
	writeBefore(t, filepath.Join(root, "z.txt"), "local\n")
	plan, err := bootstrap.PlanTemplates(root, []assets.Rendered{
		templateFile("z.txt", "desired\n", assets.OwnedByKit),
		templateFile("a.txt", "desired\n", assets.OwnedByKit),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.LocalEdits) != 2 {
		t.Fatalf("local edits before accept = %+v", plan.LocalEdits)
	}
	if acceptErr := plan.Accept([]string{"z.txt"}); acceptErr != nil {
		t.Fatal(acceptErr)
	}
	desired := install.Digest([]byte("desired\n"))
	wantManaged := []policy.ManagedFile{{Path: "a.txt", SHA256: baseline}, {Path: "z.txt", SHA256: desired}}
	wantEdits := []bootstrap.LocalEdit{{
		Path: "a.txt", Recorded: baseline, Current: install.Digest([]byte("local\n")), Proposed: desired,
	}}
	if !slices.Equal(plan.Managed, wantManaged) || !slices.Equal(plan.LocalEdits, wantEdits) {
		t.Fatalf("managed = %+v, local edits = %+v", plan.Managed, plan.LocalEdits)
	}
	if len(plan.Proposed) != 1 || plan.Proposed[0].Path != "a.txt" ||
		len(plan.Changes) != 1 || plan.Changes[0].Path != "z.txt" {
		t.Fatalf("proposed = %+v, changes = %+v", plan.Proposed, plan.Changes)
	}
}

func TestTemplatePlanAcceptSortsWritesAndRecordsRegardlessOfSelectionOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeBefore(t, filepath.Join(root, "a.txt"), "local\n")
	writeBefore(t, filepath.Join(root, "z.txt"), "local\n")
	plan, err := bootstrap.PlanTemplates(root, []assets.Rendered{
		templateFile("z.txt", "desired\n", assets.OwnedByKit),
		templateFile("m.txt", "desired\n", assets.OwnedByKit),
		templateFile("a.txt", "desired\n", assets.OwnedByKit),
	})
	if err != nil {
		t.Fatal(err)
	}
	if acceptErr := plan.Accept([]string{"z.txt", "a.txt"}); acceptErr != nil {
		t.Fatal(acceptErr)
	}
	writes := make([]string, 0, len(plan.Changes))
	records := make([]string, 0, len(plan.Managed))
	for _, change := range plan.Changes {
		writes = append(writes, change.Path)
	}
	for _, record := range plan.Managed {
		records = append(records, record.Path)
	}
	want := []string{"a.txt", "m.txt", "z.txt"}
	if !slices.Equal(writes, want) || !slices.Equal(records, want) || len(plan.Proposed) != 0 {
		t.Fatalf("writes = %v, records = %v, proposals = %+v", writes, records, plan.Proposed)
	}
}

func TestTemplatePlanAcceptNamesAnEmptyProposalSet(t *testing.T) {
	t.Parallel()
	var plan bootstrap.TemplatePlan
	if err := plan.Accept(nil); err != nil {
		t.Fatal(err)
	}
	err := plan.Accept([]string{"go.sum"})
	if !errors.Is(err, bootstrap.ErrAccept) || !strings.Contains(err.Error(), "the proposals of this run are: none") {
		t.Fatalf("accept with no proposals = %v", err)
	}
	if len(plan.Changes) != 0 || len(plan.Managed) != 0 {
		t.Fatalf("refused accept changed the plan: %+v", plan)
	}
}

func TestTemplatePlanAcceptPreservesTheFrozenInputBeforeAnyWrite(t *testing.T) {
	t.Parallel()
	root, plan := acceptPlan(t, "all:\n\techo\n")
	if err := plan.Accept([]string{".golangci.yml"}); err != nil {
		t.Fatal(err)
	}
	writeBefore(t, filepath.Join(root, ".golangci.yml"), "changed after accept\n")
	var out strings.Builder
	if err := bootstrap.ApplyTemplates(root, plan, &out); !errors.Is(err, install.ErrChanged) {
		t.Fatalf("apply after accepted input changed = %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("invalid accepted plan produced output: %q", out.String())
	}
	requireTemplateAbsent(t, root, "golaw.mk")
	for name, want := range map[string]string{
		".golangci.yml": "changed after accept\n", "Makefile": "all:\n\techo\n",
	} {
		got, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(got) != want {
			t.Fatalf("%s after refusal = %q, %v", name, got, err)
		}
	}
}

func TestTemplatePlanAcceptEquivalentNamesOncePerCall(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, destination string
		paths             []string
		managed           int
	}{
		{"repeated kit", ".golangci.yml", []string{".golangci.yml", ".golangci.yml"}, 1},
		{"equivalent kit", ".golangci.yml", []string{".golangci.yml", "./.golangci.yml"}, 1},
		{"relative Makefile", "Makefile", []string{"./Makefile"}, 0},
		{"repeated Makefile", "Makefile", []string{"Makefile", "./Makefile", "Makefile"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, plan := acceptPlan(t, "all:\n\techo\n")
			if err := plan.Accept(tc.paths); err != nil {
				t.Fatal(err)
			}
			requireAcceptedDestination(t, plan, tc.destination, tc.managed)
			if err := plan.Accept(tc.paths); !errors.Is(err, bootstrap.ErrAccept) {
				t.Fatalf("another call accepted a destination without a proposal: %v", err)
			}
		})
	}
}

func requireAcceptedDestination(t *testing.T, plan bootstrap.TemplatePlan, destination string, wantManaged int) {
	t.Helper()
	changes, managed := 0, 0
	for _, change := range plan.Changes {
		if change.Path == destination {
			changes++
		}
	}
	for _, record := range plan.Managed {
		if record.Path == destination {
			managed++
		}
	}
	if changes != 1 || managed != wantManaged ||
		slices.ContainsFunc(plan.Proposed, func(c install.Change) bool { return c.Path == destination }) {
		t.Fatalf("accepted destination: writes=%d, records=%d, proposed=%+v", changes, managed, plan.Proposed)
	}
}

func TestTemplatePlanAcceptNormalizesRefusals(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, makefile, path, want string }{
		{"module", "all:\n\techo\n", "./go.mod", "go.mod stays the person's file: apply its proposal with go get or go mod edit"},
		{"unknown", "all:\n\techo\n", "./go.sum", "go.sum; the proposals of this run are: .golangci.yml, Makefile"},
		{"collision", "lint:\n\techo\n", "./Makefile", "Makefile while lint (Makefile:1) also define golaw.mk targets"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, plan := acceptPlan(t, tc.makefile)
			err := plan.Accept([]string{tc.path})
			if !errors.Is(err, bootstrap.ErrAccept) || err.Error() != bootstrap.ErrAccept.Error()+": "+tc.want {
				t.Fatalf("Accept(%s) = %v, want %q", tc.path, err, tc.want)
			}
		})
	}
}

func TestTemplatePlanAcceptUsesNativePathSeparators(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "config"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeBefore(t, filepath.Join(root, "config", "lint.yml"), "local\n")
	plan, err := bootstrap.PlanTemplates(root, []assets.Rendered{
		templateFile("config/lint.yml", "desired\n", assets.OwnedByKit),
	})
	if err != nil {
		t.Fatal(err)
	}
	if acceptErr := plan.Accept([]string{filepath.FromSlash("./config/lint.yml"), "config/lint.yml"}); acceptErr != nil {
		t.Fatal(acceptErr)
	}
	if len(plan.Proposed) != 0 || len(plan.Changes) != 1 || plan.Changes[0].Path != "config/lint.yml" ||
		!slices.Equal(plan.Managed, []policy.ManagedFile{{Path: "config/lint.yml", SHA256: install.Digest([]byte("desired\n"))}}) {
		t.Fatalf("native destination not accepted once: %+v", plan)
	}
}

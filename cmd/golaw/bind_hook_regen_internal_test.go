package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/apply"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/source"
)

const (
	regenPortsPath = "internal/application/growth/ports.go"
	regenFakePath  = "internal/application/growth/ledgertest/zz_generated_fake.go"
)

func regenBinding(t *testing.T) hookBinding {
	t.Helper()
	h, _ := toolBinding(t)
	h.cache, h.now = source.NewCache(), now()
	return h
}

func regenPolicy() *model.EffectivePolicy {
	p := *proof.Policy()
	p.KitVersion = "0.3.0"
	return &p
}

// regenRepository is the growth fixture after golaw new port growth.Ledger
// --usecase ReadPlan, written by the engine itself.
func regenRepository(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gentest.Module(t, gentest.Contracts())
	snap, err := (binding{}).snapshot(t.Context(), repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	in, err := hookGenInput(snap, regenPolicy(), manifest.Manifest{Schema: manifest.Schema}, nil)
	if err != nil {
		t.Fatal(err)
	}
	in.Main = "slice"
	res, err := gen.New(t.Context(), in, genkind.KindPort, genkind.Request{Name: genkind.Name{Area: "growth", Type: "Ledger"}, Usecase: "ReadPlan"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := apply.Files(repo.Root, res.Writes); err != nil {
		t.Fatal(err)
	}
	return repo
}

func writeHookRepoFile(t *testing.T, root, path, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readRepoFile(t *testing.T, root, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func hasPrefixLine(lines []string, prefix string) bool {
	return slices.ContainsFunc(lines, func(line string) bool { return strings.HasPrefix(line, prefix) })
}

func TestRegenerateAfterAPortEditAppendsTheHoleAndNamesIt(t *testing.T) {
	t.Parallel()
	repo := regenRepository(t)
	ports := readRepoFile(t, repo.Root, regenPortsPath)
	edited := strings.Replace(ports, "\tArchivePlan(ctx context.Context, account AccountID) error\n",
		"\tArchivePlan(ctx context.Context, account AccountID) error\n\t// PurgePlans removes every archived plan.\n\tPurgePlans(ctx context.Context) error\n", 1)
	writeHookRepoFile(t, repo.Root, regenPortsPath, edited)
	report := regenBinding(t).regenerate(t.Context(), hook.Repository{Root: repo.Root}, regenPolicy(), []string{regenPortsPath})
	if len(report.Lines) == 0 || !strings.HasPrefix(report.Lines[0], "golaw regenerated from "+regenPortsPath+" (port Ledger): ") {
		t.Fatalf("lines = %q", report.Lines)
	}
	if !hasPrefixLine(report.Lines, "hole open internal/application/growth/ledgertest/cases.go:") || !slices.ContainsFunc(report.Lines, func(l string) bool { return strings.Contains(l, " ledger.purge-plans ") }) {
		t.Fatalf("no open hole for the new method in %q", report.Lines)
	}
	if !strings.Contains(readRepoFile(t, repo.Root, regenFakePath), "PurgePlans") {
		t.Fatalf("the fake was not regenerated")
	}
	if len(report.Person) != len(report.Lines)+len(report.Notices) || !strings.HasPrefix(report.Person[0], "golaw regenerou a partir de "+regenPortsPath+" (port Ledger): ") {
		t.Fatalf("person = %q", report.Person)
	}
}

func TestRegenerateFeedsAHandEditOfARegenerableFileBack(t *testing.T) {
	t.Parallel()
	repo := regenRepository(t)
	writeHookRepoFile(t, repo.Root, regenFakePath, readRepoFile(t, repo.Root, regenFakePath)+"\n// edited by hand\n")
	report := regenBinding(t).regenerate(t.Context(), hook.Repository{Root: repo.Root}, regenPolicy(), []string{regenFakePath})
	want := "drift " + regenFakePath + " " + gen.DriftDiffers + ": edit internal/application/growth/ports.go, or run golaw gen"
	if !slices.Contains(report.Lines, want) {
		t.Fatalf("lines = %q, want %q", report.Lines, want)
	}
	if !strings.Contains(readRepoFile(t, repo.Root, regenFakePath), "// edited by hand") {
		t.Fatalf("the hook restored the file (A46)")
	}
}

func TestRegenerateDefersWhileAnotherHookHoldsTheLock(t *testing.T) {
	t.Parallel()
	repo := regenRepository(t)
	found, err := source.Discover(t.Context(), repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := runner.Acquire(t.Context(), runner.LockPath(found.GitDir, genLockName), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lock.Release(); err != nil {
			t.Error(err)
		}
	}()
	report := regenBinding(t).regenerate(t.Context(), hook.Repository{Root: repo.Root}, regenPolicy(), []string{regenPortsPath})
	if !slices.Equal(report.Notices, []string{"regeneration deferred: run golaw gen"}) || len(report.Lines) != 0 || report.Person[len(report.Person)-1] != "golaw adiou a regeneração: rode golaw gen." {
		t.Fatalf("report = %+v", report)
	}
}

func TestRegenerateAppliesTheCertainFixesToTheGoFilesTheEventWrote(t *testing.T) {
	t.Parallel()
	repo := regenRepository(t)
	const policyFile = "internal/application/growth/policy.go"
	const broken = "internal/application/growth/broken.go"
	writeHookRepoFile(t, repo.Root, policyFile, "package growth\n\nimport \"strings\"\n\nfunc label( ) string {return \"x\"}\n")
	writeHookRepoFile(t, repo.Root, broken, "package growth\n\nfunc {\n")
	writeHookRepoFile(t, repo.Root, "README.md", "# slice\n")
	report := regenBinding(t).regenerate(t.Context(), hook.Repository{Root: repo.Root}, regenPolicy(), []string{policyFile, broken, "README.md"})
	if got := readRepoFile(t, repo.Root, policyFile); got != "package growth\n\nfunc label() string { return \"x\" }\n" {
		t.Fatalf("policy.go =\n%s", got)
	}
	if !slices.Contains(report.Lines, "formatted "+policyFile+": go/format, imports fixed") || !hasPrefixLine(report.Lines, "parse "+broken+":3:") {
		t.Fatalf("lines = %q", report.Lines)
	}
	if readRepoFile(t, repo.Root, broken) != "package growth\n\nfunc {\n" {
		t.Fatalf("a file that does not parse was changed")
	}
	if empty := regenBinding(t).regenerate(t.Context(), hook.Repository{Root: repo.Root}, regenPolicy(), []string{"README.md"}); len(empty.Lines)+len(empty.Notices) != 0 {
		t.Fatalf("a write that is no Go file and no contract answered %+v", empty)
	}
}

func TestRegenerateWithoutAManifestRegeneratesNothing(t *testing.T) {
	t.Parallel()
	repo := gentest.Module(t, gentest.Contracts())
	report := regenBinding(t).regenerate(t.Context(), hook.Repository{Root: repo.Root}, regenPolicy(), []string{regenPortsPath})
	if len(report.Lines)+len(report.Notices) != 0 {
		t.Fatalf("report = %+v", report)
	}
}

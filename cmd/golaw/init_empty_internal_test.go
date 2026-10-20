package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestInitWithoutModuleOnlyWritesPolicyAndEnrollment(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ language, message string }{
		{"pt-BR", "Execute go mod init <module path> antes de gerar os templates."},
		{"en", "Run go mod init <module path> before generating templates."},
	} {
		t.Run(tc.language, func(t *testing.T) {
			t.Parallel()
			initWithoutModule(t, tc.language, tc.message)
		})
	}
}

// initWithoutModule applies init to a repository without go.mod and
// requires the policy and the enrollment alone, with the instruction to
// create the module.
func initWithoutModule(t *testing.T, language, message string) {
	t.Helper()
	repo := gittest.New(t)
	ta := initBindApp(t)
	args := []string{"init", "--apply", "--language", language, repo.Root}
	if code := ta.run(t.Context(), args); code != exitClean {
		t.Fatalf("exit = %d: %s", code, ta.stderr.String())
	}
	if !strings.Contains(ta.stdout.String(), message+"\n") {
		t.Fatalf("module instruction missing: %q", ta.stdout.String())
	}
	entries, err := os.ReadDir(repo.Root)
	if err != nil || len(entries) != 2 {
		t.Fatalf("unexpected files: %+v, %v", entries, err)
	}
	for _, entry := range entries {
		if entry.Name() != ".git" && entry.Name() != "golaw.yml" {
			t.Fatalf("template written without module: %s", entry.Name())
		}
	}
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	if _, found, readErr := policy.ReadRecord(dirs.State, repo.Root); readErr != nil || !found {
		t.Fatalf("enrollment = %v, %v", found, readErr)
	}
}

func TestInitOfAnOlderGoDirectiveOnlyWritesPolicyAndEnrollment(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ language, message string }{
		{"pt-BR", "Atualize a diretiva go do go.mod para 1.24 ou mais nova antes de gerar os templates."},
		{"en", "Raise the go directive of go.mod to 1.24 or newer before generating templates."},
	} {
		t.Run(tc.language, func(t *testing.T) {
			t.Parallel()
			initOlderModule(t, tc.language, tc.message)
		})
	}
}

// initOlderModule applies init to a module on go 1.22 and requires the
// policy and the enrollment without a template or a change to go.mod.
func initOlderModule(t *testing.T, language, message string) {
	t.Helper()
	const module = "module example.test/service\n\ngo 1.22\n"
	repo := gittest.New(t)
	repo.Write("go.mod", module)
	ta := initBindApp(t)
	if code := ta.run(t.Context(), []string{"init", "--apply", "--language", language, repo.Root}); code != exitClean {
		t.Fatalf("exit = %d: %s", code, ta.stderr.String())
	}
	if !strings.Contains(ta.stdout.String(), message+"\n") {
		t.Fatalf("go floor instruction missing: %q", ta.stdout.String())
	}
	entries, err := os.ReadDir(repo.Root)
	if err != nil || len(entries) != 3 {
		t.Fatalf("a template was written below the go floor: %+v, %v", entries, err)
	}
	if got, readErr := os.ReadFile(filepath.Join(repo.Root, "go.mod")); readErr != nil || string(got) != module {
		t.Fatalf("go.mod changed: %q, %v", got, readErr)
	}
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	if _, found, readErr := policy.ReadRecord(dirs.State, repo.Root); readErr != nil || !found {
		t.Fatalf("enrollment = %v, %v", found, readErr)
	}
}

func TestInitOfAnUnreadableModuleOnlyWritesPolicyAndEnrollment(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ language, prefix, suffix string }{
		{
			"pt-BR", "O comando go não leu go.mod (",
			"); coloque go no PATH ou corrija go.mod e execute golaw init de novo antes de gerar os templates.\n",
		},
		{
			"en", "The go command could not read go.mod (",
			"); put go on PATH or repair go.mod, then run golaw init again before generating templates.\n",
		},
	} {
		t.Run(tc.language, func(t *testing.T) {
			t.Parallel()
			initUnreadableModule(t, tc.language, tc.prefix, tc.suffix)
		})
	}
}

// initUnreadableModule applies init to a module the go command cannot
// read and requires the policy and the enrollment of M1, one line naming
// the cause and the fix, and no template or change to go.mod.
func initUnreadableModule(t *testing.T, language, prefix, suffix string) {
	t.Helper()
	const module = "module example.test/service\n\ngo 1.27\n\nbogus directive\n"
	repo := gittest.New(t)
	repo.Write("go.mod", module)
	ta := initBindApp(t)
	if code := ta.run(t.Context(), []string{"init", "--apply", "--language", language, repo.Root}); code != exitClean {
		t.Fatalf("exit = %d: %s", code, ta.stderr.String())
	}
	line := ""
	for candidate := range strings.SplitSeq(ta.stdout.String(), "\n") {
		if strings.HasPrefix(candidate, prefix) {
			line = candidate
		}
	}
	if !strings.HasSuffix(line+"\n", suffix) || !strings.Contains(line, "unknown directive: bogus") {
		t.Fatalf("unreadable module line missing: %q", ta.stdout.String())
	}
	entries, err := os.ReadDir(repo.Root)
	if err != nil || len(entries) != 3 {
		t.Fatalf("a template was written for an unreadable module: %+v, %v", entries, err)
	}
	if got, readErr := os.ReadFile(filepath.Join(repo.Root, "go.mod")); readErr != nil || string(got) != module {
		t.Fatalf("go.mod changed: %q, %v", got, readErr)
	}
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	if _, found, readErr := policy.ReadRecord(dirs.State, repo.Root); readErr != nil || !found {
		t.Fatalf("enrollment = %v, %v", found, readErr)
	}
}

func TestInitOutputFailureDoesNotWritePolicyOrTemplates(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Write("go.mod", "module example.test/service\n\ngo 1.27\n")
	ta := initBindApp(t)
	reader, writer := io.Pipe()
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	ta.app.stdout = writer
	code := ta.run(t.Context(), []string{"init", "--apply", repo.Root})
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if code != exitFailure {
		t.Fatalf("exit = %d", code)
	}
	entries, err := os.ReadDir(repo.Root)
	if err != nil || len(entries) != 2 {
		t.Fatalf("partial initialization: %+v, %v", entries, err)
	}
	if _, err := os.Stat(filepath.Join(repo.Root, "go.mod")); err != nil {
		t.Fatal(err)
	}
}

package main

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
)

const acceptLint = "version: 2\n\nlinters:\n  enable:\n    - errcheck\n"

// acceptBirth commits a repository born with its own lint configuration and
// the Makefile given.
func acceptBirth(t *testing.T, makefile string) *gittest.Repo {
	t.Helper()
	repo := gittest.New(t)
	repo.Commit("chore: establish application", map[string]string{
		"go.mod": "module example.test/birth\n\ngo 1.27\n", "Makefile": makefile, ".golangci.yml": acceptLint,
	})
	return repo
}

func TestInitAcceptWritesOnePrintedProposal(t *testing.T) {
	t.Parallel()
	repo := acceptBirth(t, "lint-template:\n\tgolangci-lint run\n")
	ta := initBindApp(t)
	if code := ta.run(t.Context(), []string{"init", "--apply", "--accept", ".golangci.yml", repo.Root}); code != exitClean {
		t.Fatalf("accept exit = %d: %s", code, ta.stderr.String())
	}
	written, err := os.ReadFile(filepath.Join(repo.Root, ".golangci.yml"))
	if err != nil || string(written) == acceptLint {
		t.Fatalf("the strict asset was not written: %q, %v", written, err)
	}
	data, err := os.ReadFile(filepath.Join(repo.Root, "golaw.yml"))
	if err != nil {
		t.Fatal(err)
	}
	config, err := policy.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(config.Managed, policy.ManagedFile{Path: ".golangci.yml", SHA256: install.Digest(written)}) {
		t.Fatalf("managed = %+v", config.Managed)
	}
	makefile, err := os.ReadFile(filepath.Join(repo.Root, "Makefile"))
	if err != nil || string(makefile) != "lint-template:\n\tgolangci-lint run\n" {
		t.Fatalf("a proposal nobody accepted was written: %q, %v", makefile, err)
	}
	ta.stdout.Reset()
	if code := ta.run(t.Context(), []string{"init", "--apply", repo.Root}); code != exitClean {
		t.Fatalf("second apply exit = %d: %s", code, ta.stderr.String())
	}
	if strings.Contains(ta.stdout.String(), "+++ .golangci.yml") {
		t.Fatalf("the accepted file is proposed again:\n%s", ta.stdout.String())
	}
}

func TestInitAcceptRefusesBeforeAnyWrite(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, makefile string
		args           []string
		want           string
	}{
		{"red not a proposal", "lint-template:\n\techo\n", []string{"--apply", "--accept", "go.sum"}, "the proposals of this run are: .golangci.yml, Makefile"},
		{"red collision", "lint:\n\techo\n", []string{"--apply", "--accept", "Makefile"}, "lint (Makefile:1)"},
		{"red with update", "lint-template:\n\techo\n", []string{"--update", "--accept", "Makefile"}, "--update"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := acceptBirth(t, tc.makefile)
			ta := initBindApp(t)
			args := append(append([]string{"init"}, tc.args...), repo.Root)
			if code := ta.run(t.Context(), args); code != exitFailure || !strings.Contains(ta.stderr.String(), tc.want) {
				t.Fatalf("exit = %d, stderr %q, want %q", code, ta.stderr.String(), tc.want)
			}
			if _, err := os.Stat(filepath.Join(repo.Root, "golaw.yml")); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("a refused accept wrote golaw.yml: %v", err)
			}
		})
	}
	repo := acceptBirth(t, "lint-template:\n\techo\n")
	ta := initBindApp(t)
	if code := ta.run(t.Context(), []string{"init", "--apply", "--accept", "Makefile", repo.Root}); code != exitClean {
		t.Fatalf("near accept exit = %d: %s", code, ta.stderr.String())
	}
	makefile, err := os.ReadFile(filepath.Join(repo.Root, "Makefile"))
	if err != nil || !strings.HasSuffix(string(makefile), "include golaw.mk\n") {
		t.Fatalf("near accept left %q, %v", makefile, err)
	}
}

func TestInitAcceptPreviewLeavesFilesAndEnrollmentUntouched(t *testing.T) {
	t.Parallel()
	repo := acceptBirth(t, "all:\n\techo\n")
	before := ownershipTree(t, repo.Root)
	ta := initBindApp(t)
	args := []string{"init", "--accept", "Makefile", "--accept", ".golangci.yml", repo.Root}
	if code := ta.run(t.Context(), args); code != exitClean {
		t.Fatalf("preview exit = %d: %s", code, ta.stderr.String())
	}
	_, writes, found := strings.Cut(ta.stdout.String(), "Gravações:\n")
	writes, _, _ = strings.Cut(writes, "Propostas sem gravação:\n")
	for _, path := range []string{"Makefile", ".golangci.yml"} {
		if !found || !strings.Contains(writes, "+++ "+path+"\n") {
			t.Fatalf("accepted proposal missing from writes: %s", ta.stdout.String())
		}
	}
	if !maps.Equal(before, ownershipTree(t, repo.Root)) {
		t.Fatal("preview changed files")
	}
	requireOwnershipEnrollment(t, ta, repo.Root, nil, false)
	requireUnarmed(t, repo.GitDir())
}

func TestInitAcceptUnavailableProposalsPreserveEveryFile(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		prepare func(*gittest.Repo)
		paths   []string
		want    string
	}{
		{
			"no module", func(repo *gittest.Repo) { repo.Remove("go.mod") },
			[]string{".golangci.yml"},
			"init rendered no template",
		},
		{
			"old module", func(repo *gittest.Repo) { repo.Write("go.mod", "module example.test/birth\n\ngo 1.23\n") },
			[]string{".golangci.yml"},
			"init rendered no template",
		},
		{"protected destination", func(repo *gittest.Repo) {
			repo.Commit("chore: protect lint settings", map[string]string{"CODEOWNERS": "* @core\n/.golangci.yml @platform\n"})
		}, []string{".golangci.yml"}, "the proposals of this run are: Makefile"},
		{"later invalid path", nil, []string{".golangci.yml", "go.sum"}, "the proposals of this run are: Makefile"},
		{"module is personal", nil, []string{"go.mod"}, "go get or go mod edit"},
		{"normalized module is personal", nil, []string{"./go.mod"}, "go get or go mod edit"},
		{
			"normalized invalid path", nil,
			[]string{"nested/../go.sum"},
			"not a proposal init can write: go.sum; the proposals of this run are: .golangci.yml, Makefile",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := acceptBirth(t, "all:\n\techo\n")
			if tc.prepare != nil {
				tc.prepare(repo)
			}
			before := ownershipTree(t, repo.Root)
			ta := initBindApp(t)
			args := []string{"init", "--apply", "--accept", strings.Join(tc.paths, ","), repo.Root}
			if code := ta.run(t.Context(), args); code != exitFailure || !strings.Contains(ta.stderr.String(), tc.want) {
				t.Fatalf("exit = %d, stderr %q, want %q", code, ta.stderr.String(), tc.want)
			}
			if !maps.Equal(before, ownershipTree(t, repo.Root)) || ta.stdout.Len() != 0 {
				t.Fatal("refused acceptance changed files or printed partial output")
			}
			requireOwnershipEnrollment(t, ta, repo.Root, nil, false)
			requireUnarmed(t, repo.GitDir())
		})
	}
}

func TestInitAcceptHoldsChangedRecordForProtectedPolicy(t *testing.T) {
	t.Parallel()
	repo, original := protectedPolicyFixture(t)
	config, err := policy.Decode(original)
	if err != nil {
		t.Fatal(err)
	}
	index := slices.IndexFunc(config.Managed, func(m policy.ManagedFile) bool { return m.Path == ".golangci.yml" })
	if index < 0 {
		t.Fatal("fixture has no lint record")
	}
	config.Managed[index].SHA256 = install.Digest([]byte(acceptLint))
	before, err := bootstrap.ManagedPolicy(original, config.Kit.Version, config.Managed)
	if err != nil {
		t.Fatal(err)
	}
	repo.Write(bootstrap.PolicyFile, string(before))
	repo.Write(".golangci.yml", acceptLint)
	snapshot := ownershipTree(t, repo.Root)
	ta := initBindApp(t)
	if code := ta.run(t.Context(), []string{"init", "--apply", "--accept", ".golangci.yml", repo.Root}); code != exitClean {
		t.Fatalf("protected policy exit = %d: %s", code, ta.stderr.String())
	}
	if !strings.Contains(ta.stdout.String(), heldRecordPrefix+".golangci.yml sha256=") {
		t.Fatalf("held record missing: %s", ta.stdout.String())
	}
	if !maps.Equal(snapshot, ownershipTree(t, repo.Root)) {
		t.Fatal("accepted template changed a protected policy or its file")
	}
	requireOwnershipEnrollment(t, ta, repo.Root, before, true)
	requireUnarmed(t, repo.GitDir())
}

func TestInitAcceptNormalizesPathsAndIgnoresRepeatedSelections(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path string
		args       []string
	}{
		{"repeated flags", ".golangci.yml", []string{"--accept", ".golangci.yml", "--accept", ".golangci.yml"}},
		{"comma repeat", ".golangci.yml", []string{"--accept", ".golangci.yml,.golangci.yml"}},
		{"normalized duplicate", ".golangci.yml", []string{"--accept", "./.golangci.yml,nested/../.golangci.yml"}},
		{"normalized merged path", "Makefile", []string{"--accept", "./Makefile"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			const originalMakefile = "all:\n\techo\n"
			repo := acceptBirth(t, originalMakefile)
			ta := initBindApp(t)
			args := append(append([]string{"init", "--apply"}, tc.args...), repo.Root)
			if code := ta.run(t.Context(), args); code != exitClean {
				t.Fatalf("normalized accept exit = %d: %s", code, ta.stderr.String())
			}
			written, err := os.ReadFile(filepath.Join(repo.Root, tc.path))
			if err != nil {
				t.Fatal(err)
			}
			if tc.path == "Makefile" {
				if string(written) != originalMakefile+"include golaw.mk\n" {
					t.Fatalf("normalized merged proposal = %q", written)
				}
			} else if string(written) == acceptLint {
				t.Fatalf("normalized kit proposal was not written: %q", written)
			}
			requireUnarmed(t, repo.GitDir())
		})
	}
}

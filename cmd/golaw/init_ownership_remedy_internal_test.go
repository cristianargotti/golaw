package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
)

const (
	protectedPolicyOwners = "* @core\n/golaw.yml @platform\n"
	protectedPolicyRemedy = "one of its owners must commit golaw.yml and the files golaw init previews, " +
		"then golaw init --apply enrolls it"
	heldRecordPrefix = "Template sem registro, CODEOWNERS protege golaw.yml: "
)

func TestInitProtectedPolicyRemedyEnrolls(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Commit("chore: establish ownership", map[string]string{
		"go.mod": "module example.test/service\n\ngo 1.27\n", "CODEOWNERS": protectedPolicyOwners,
	})
	ta := initBindApp(t)
	if code := ta.run(t.Context(), []string{"init", repo.Root}); code != exitClean {
		t.Fatalf("preview of a protected policy exit = %d: %s", code, ta.stderr.String())
	}
	preview := ta.stdout.String()
	if !strings.Contains(preview, "init: nothing written; --apply would refuse: ") ||
		!strings.Contains(preview, protectedPolicyRemedy) || !strings.Contains(preview, "+++ golaw.yml\n") {
		t.Fatalf("preview of a protected policy = %q", preview)
	}
	requireNoInitTemplates(t, repo.Root)
	ta.stdout.Reset()
	if code := ta.run(t.Context(), []string{"init", "--apply", repo.Root}); code != exitFailure ||
		!strings.Contains(ta.stderr.String(), protectedPolicyRemedy) || ta.stdout.Len() != 0 {
		t.Fatalf("protected policy apply exit = %d: %q, %q", code, ta.stdout.String(), ta.stderr.String())
	}
	files := previewedFiles(preview)
	if _, found := files[bootstrap.PolicyFile]; !found || len(files) < 2 {
		t.Fatalf("previewed files = %v", files)
	}
	repo.Commit("chore: adopt the previewed policy and templates", files)
	ta.stdout.Reset()
	if code := ta.run(t.Context(), []string{"init", "--apply", repo.Root}); code != exitClean {
		t.Fatalf("remedy refused: exit = %d: %s", code, ta.stderr.String())
	}
	requireRemedyEnrolled(t, ta, repo.Root, files[bootstrap.PolicyFile], true)
}

func TestInitKeepsAProtectedPolicyAndHoldsNewRecords(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	reviewed := m1Policy(t)
	repo.Commit("chore: adopt the policy", map[string]string{
		"go.mod":    adoptedModule,
		"golaw.yml": reviewed, "CODEOWNERS": protectedPolicyOwners,
	})
	ta := initBindApp(t)
	if code := ta.run(t.Context(), []string{"init", "--apply", repo.Root}); code != exitClean {
		t.Fatalf("protected reviewed policy refused: exit = %d: %s", code, ta.stderr.String())
	}
	out := ta.stdout.String()
	held := heldRecords(out)
	if len(held) == 0 || !strings.Contains(out, "Propostas sem gravação:\n") {
		t.Fatalf("held records not reported: %q", out)
	}
	requireTemplatePresence(t, repo.Root, "lefthook.yml", false)
	requireTemplatePresence(t, repo.Root, ".editorconfig", true)
	requireRemedyEnrolled(t, ta, repo.Root, reviewed, false)
	files := previewedFiles(out)
	recorded := make(map[string]string, len(held)+1)
	for _, record := range held {
		content, found := files[record.Path]
		if !found || install.Digest([]byte(content)) != record.SHA256 {
			t.Fatalf("held file %s not proposed with its hash: %q", record.Path, content)
		}
		recorded[record.Path] = content
	}
	data, err := bootstrap.ManagedPolicy([]byte(reviewed), testVersion, held)
	if err != nil {
		t.Fatal(err)
	}
	recorded[bootstrap.PolicyFile] = string(data)
	repo.Git("add", "--", ".")
	repo.Commit("chore: record the kit templates", recorded)
	ta.stdout.Reset()
	if code := ta.run(t.Context(), []string{"init", "--apply", repo.Root}); code != exitClean {
		t.Fatalf("recorded templates refused: exit = %d: %s", code, ta.stderr.String())
	}
	if strings.Contains(ta.stdout.String(), heldRecordPrefix) {
		t.Fatalf("records still held after the remedy: %q", ta.stdout.String())
	}
	requireRemedyEnrolled(t, ta, repo.Root, string(data), true)
}

// requireRemedyEnrolled requires golaw.yml to hold the bytes an owner
// committed and the record to enroll them; once settled, init writes and
// proposes nothing more.
func requireRemedyEnrolled(t *testing.T, ta testApp, root, committed string, settled bool) {
	t.Helper()
	data, err := bootstrap.Current(root)
	if err != nil || string(data) != committed {
		t.Fatalf("protected policy changed: %q, %v", data, err)
	}
	out := ta.stdout.String()
	if strings.Contains(out, "+++ golaw.yml") || !strings.Contains(out, "Destino protegido por CODEOWNERS, preservado: golaw.yml.\n") {
		t.Fatalf("protected policy output = %q", out)
	}
	if settled && (strings.Contains(out, "Gravações:") || strings.Contains(out, "Propostas sem gravação:")) {
		t.Fatalf("an adopted policy still writes or proposes: %q", out)
	}
	requireOwnershipEnrollment(t, ta, root, data, true)
}

func requireTemplatePresence(t *testing.T, root, name string, want bool) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); (err == nil) != want {
		t.Fatalf("%s present = %v, want %v: %v", name, err == nil, want, err)
	}
}

// previewedFiles reads back the new files a diff prints: a "+++ path"
// header opens one and every "+" line under it is one of its lines.
func previewedFiles(out string) map[string]string {
	files := make(map[string]string)
	path := ""
	for line := range strings.SplitSeq(out, "\n") {
		if header, found := strings.CutPrefix(line, "+++ "); found {
			path, files[header] = header, ""
			continue
		}
		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "@@ ") {
			continue
		}
		if added, found := strings.CutPrefix(line, "+"); found && path != "" {
			files[path] += added + "\n"
			continue
		}
		path = ""
	}
	return files
}

// heldRecords reads the path and hash of every record a protected policy
// held back.
func heldRecords(out string) []policy.ManagedFile {
	var held []policy.ManagedFile
	for line := range strings.SplitSeq(out, "\n") {
		rest, found := strings.CutPrefix(line, heldRecordPrefix)
		if !found {
			continue
		}
		record, _, _ := strings.Cut(rest, ";")
		path, hash, _ := strings.Cut(record, " sha256=")
		held = append(held, policy.ManagedFile{Path: path, SHA256: hash})
	}
	return held
}

// m1Policy is the golaw.yml init of M1 wrote for a Go module: the core
// defaults with an empty managed list.
func m1Policy(t *testing.T) string {
	t.Helper()
	in, err := (bootstrap.Inputs{
		KitVersion: testVersion, Language: "pt-BR",
		Layout: bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/adr", Go: true},
	}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	data, err := bootstrap.Render(in)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

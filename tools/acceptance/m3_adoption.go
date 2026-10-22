package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	scanreport "github.com/cristianargotti/golaw/internal/report"
)

const (
	m3AdoptionCategory = "m3-adoption"
	m3AcceptFlag       = "--accept"
	m3LintConfig       = ".golangci.yml"
	m3Makefile         = "Makefile"
	m3EditorConfig     = ".editorconfig"
	m3GolawMk          = "golaw.mk"
	// m3OlderGo is under the go floor of the templates and m3GoFloor is the
	// floor init names, so the row proves init renders nothing below it.
	m3OlderGo = "go 1.23"
	m3GoFloor = "1.24"
)

// m3GoDirective is the go line of a go.mod; the toolchain line never
// matches, because it starts with toolchain.
var m3GoDirective = regexp.MustCompile(`(?m)^go [0-9.]+$`)

// m3Mechanism is one adoption mechanism the preview of init must name,
// with the file whose proposal must hold it when only one may.
type m3Mechanism struct {
	name, proposal, fragment string
}

func m3PreviewMechanisms() []m3Mechanism {
	return []m3Mechanism{
		{"the targets the Makefile and golaw.mk both define", "", "lint (Makefile:22), test (Makefile:28)"},
		{"the imported modules go.mod marks indirect", "", "github.com/spf13/cobra, github.com/stretchr/testify"},
		{"the Makefile section of the merged .editorconfig", m3EditorConfig, "+[Makefile]"},
		{"the capabilities the files suggest", "", "api (api/swagger.yaml), image (Dockerfile)"},
	}
}

// m3AdoptionCases are the adoption mechanisms of the template survey, each
// over a birth of its own, so no case reads what another one wrote.
func m3AdoptionCases() map[string]func(*suite, context.Context, string) error {
	return map[string]func(*suite, context.Context, string) error{
		"birth-accept":   (*suite).m3BirthAccept,
		"birth-older-go": (*suite).m3BirthOlderGo,
		"birth-preview":  (*suite).m3BirthPreview,
		"birth-scan":     (*suite).m3BirthScan,
	}
}

func m3AdoptionNames() []string {
	return slices.Sorted(maps.Keys(m3AdoptionCases()))
}

// adoptionM3 refuses an unknown case before any command runs, so a typo in
// a row name never leaves a birth behind.
func (s *suite) adoptionM3(ctx context.Context, name string) error {
	run, found := m3AdoptionCases()[name]
	if !found {
		return fmt.Errorf("unknown M3 adoption case %q: %w", name, ErrAcceptance)
	}
	repo, err := s.m3BirthRepository(ctx, name)
	if err != nil {
		return err
	}
	return run(s, ctx, repo)
}

// m3BirthRepository is the birth as the scaffolder leaves it: the fixture
// committed as the first commit, before golaw is installed.
func (s *suite) m3BirthRepository(ctx context.Context, name string) (string, error) {
	repo, err := s.repository(ctx, name, false)
	if err != nil {
		return "", err
	}
	if writeErr := m3WriteBirth(s.root, repo); writeErr != nil {
		return "", writeErr
	}
	steps := [][]string{{commandInit, m2Quiet}, {gitAdd, "."}, commitArgs("chore: render the service")}
	if gitErr := s.gitSteps(ctx, repo, steps...); gitErr != nil {
		return "", gitErr
	}
	return repo, nil
}

// m3BirthPreview proves init names every mechanism before it writes and,
// without --apply, writes nothing.
func (s *suite) m3BirthPreview(ctx context.Context, repo string) error {
	if err := m3JudgePreview(s.m3Golaw(ctx, repo, commandInit)); err != nil {
		return err
	}
	got := s.execute(ctx, command{program: s.git, args: []string{"status", "--porcelain"}, dir: repo, env: s.env})
	if err := success(got); err != nil {
		return err
	}
	if status := strings.TrimSpace(got.stdout); status != "" {
		return fmt.Errorf("init without --apply changed the birth: %s: %w", status, ErrAcceptance)
	}
	return nil
}

// m3JudgePreview requires a preview that exits 0 and names each mechanism,
// a fragment bound to one proposal counting only inside it.
func m3JudgePreview(got result) error {
	if err := m3Exited(got, "golaw init"); err != nil {
		return err
	}
	for _, mechanism := range m3PreviewMechanisms() {
		text := got.stdout
		if mechanism.proposal != "" {
			text = m3Proposal(text, mechanism.proposal)
		}
		if !strings.Contains(text, mechanism.fragment) {
			return fmt.Errorf("the preview of init does not name %s (%q): %w", mechanism.name, mechanism.fragment, ErrAcceptance)
		}
	}
	return nil
}

// m3Proposal is the part of a preview that proposes one file, from its new
// file header to the header of the next file, so a fragment counts only in
// the proposal it belongs to.
func m3Proposal(preview, path string) string {
	_, after, found := strings.Cut(preview, "+++ "+path+"\n")
	if !found {
		return ""
	}
	proposal, _, _ := strings.Cut(after, "\n--- ")
	return proposal
}

// m3BirthAccept proves --accept writes the one proposal it names, records
// it as managed, and leaves nothing to propose for it on the next run.
func (s *suite) m3BirthAccept(ctx context.Context, repo string) error {
	accepted := s.m3Golaw(ctx, repo, commandInit, flagApply, m3AcceptFlag, m3LintConfig)
	if err := m3Exited(accepted, "golaw init --apply --accept "+m3LintConfig); err != nil {
		return err
	}
	if err := m3AcceptedLint(repo); err != nil {
		return err
	}
	again := s.m3Golaw(ctx, repo, commandInit, flagApply)
	if err := m3Exited(again, "a second golaw init --apply"); err != nil {
		return err
	}
	if strings.Contains(again.stdout, "+++ "+m3LintConfig) {
		return fmt.Errorf("a second init --apply proposes %s again: %w", m3LintConfig, ErrAcceptance)
	}
	return nil
}

// m3AcceptedLint requires the strict asset in place of the birth's own
// configuration and its hash in the managed list, which is what lets a
// later update rewrite it.
func m3AcceptedLint(repo string) error {
	lint, err := readFile(filepath.Join(repo, m3LintConfig))
	if err != nil {
		return err
	}
	if !strings.Contains(string(lint), "default: standard") {
		return fmt.Errorf("%s after --accept is not the strict asset: %w", m3LintConfig, ErrAcceptance)
	}
	data, err := readFile(filepath.Join(repo, policyFile))
	if err != nil {
		return err
	}
	file, err := policy.Decode(data)
	if err != nil {
		return fmt.Errorf("decode %s: %w", policyFile, err)
	}
	want := policy.ManagedFile{Path: m3LintConfig, SHA256: install.Digest(lint)}
	if !slices.Contains(file.Managed, want) {
		return fmt.Errorf("%s records no managed %s at sha256 %s: %w", policyFile, m3LintConfig, want.SHA256, ErrAcceptance)
	}
	return nil
}

// m3BirthScan takes every repair init names and every proposal it prints,
// then requires a scan that measures each check: findings are allowed,
// because the birth keeps the template repairs the owner holds.
func (s *suite) m3BirthScan(ctx context.Context, repo string) error {
	path := filepath.Join(repo, m3Makefile)
	makefile, err := readFile(path)
	if err != nil {
		return err
	}
	if writeErr := writeFile(path, []byte(m3RenameCollisions(string(makefile))), fileMode); writeErr != nil {
		return writeErr
	}
	got := s.m3Golaw(ctx, repo, commandInit, flagApply, "--capabilities", "api,go,image",
		m3AcceptFlag, m3LintConfig, m3AcceptFlag, m3Makefile, m3AcceptFlag, m3EditorConfig)
	if exitErr := m3Exited(got, "golaw init --apply --capabilities api,go,image"); exitErr != nil {
		return exitErr
	}
	if gitErr := s.gitSteps(ctx, repo, []string{gitAdd, "."}, commitArgs("feat: adopt golaw on the template birth")); gitErr != nil {
		return gitErr
	}
	doc, code, err := s.m3Scan(ctx, repo)
	if err != nil {
		return err
	}
	return m3JudgeMeasured(doc, code)
}

// m3JudgeMeasured refuses a scan that left a check unmeasured, the exit 2
// the pipeline would stop at, and a document without checks, which would
// pass for nothing measured.
func m3JudgeMeasured(doc scanreport.Document, code int) error {
	if len(doc.Checks) == 0 {
		return fmt.Errorf("the scan of the adopted birth exited %d and listed no check: %w", code, ErrAcceptance)
	}
	for _, status := range doc.Checks {
		if status.Status == model.StatusUnmeasured.String() {
			return fmt.Errorf("the scan of the adopted birth left %s unmeasured: %s: %w", status.ID, status.Reason, ErrAcceptance)
		}
	}
	if doc.Counts.Unmeasured != 0 || code > 1 {
		return fmt.Errorf("the scan of the adopted birth exited %d with %d unmeasured: %w", code, doc.Counts.Unmeasured, ErrAcceptance)
	}
	return nil
}

// m3RenameCollisions renames the two birth targets golaw.mk also defines,
// the repair the preview names, and keeps their recipes.
func m3RenameCollisions(makefile string) string {
	makefile = regexp.MustCompile(`(?m)^lint:`).ReplaceAllString(makefile, "lint-template:")
	return regexp.MustCompile(`(?m)^test:`).ReplaceAllString(makefile, "test-compose:")
}

// m3BirthOlderGo proves a module under the go floor gets the floor named
// and no golaw.mk, whose targets need a newer go.
func (s *suite) m3BirthOlderGo(ctx context.Context, repo string) error {
	path := filepath.Join(repo, moduleFile)
	module, err := readFile(path)
	if err != nil {
		return err
	}
	older := m3GoDirective.ReplaceAll(module, []byte(m3OlderGo))
	if bytes.Equal(older, module) {
		return fmt.Errorf("the birth go.mod holds no go line to lower: %w", ErrAcceptance)
	}
	if writeErr := writeFile(path, older, fileMode); writeErr != nil {
		return writeErr
	}
	got := s.m3Golaw(ctx, repo, commandInit, flagApply)
	if exitErr := m3Exited(got, "golaw init --apply over "+m3OlderGo); exitErr != nil {
		return exitErr
	}
	if !strings.Contains(got.stdout+got.stderr, m3GoFloor) {
		return fmt.Errorf("init over %s does not name the floor %s: %w", m3OlderGo, m3GoFloor, ErrAcceptance)
	}
	switch _, statErr := os.Stat(filepath.Join(repo, m3GolawMk)); {
	case statErr == nil:
		return fmt.Errorf("init over %s wrote %s: %w", m3OlderGo, m3GolawMk, ErrAcceptance)
	case !errors.Is(statErr, fs.ErrNotExist):
		return fmt.Errorf("inspect %s: %w", m3GolawMk, statErr)
	}
	return nil
}

func (s *suite) m3Golaw(ctx context.Context, repo string, args ...string) result {
	return s.execute(ctx, command{program: s.binary, args: args, dir: repo, env: s.env})
}

// m3Exited names a command that did not start or did not exit 0 with the
// end of its output, which is where a failed init names its reason.
func m3Exited(got result, what string) error {
	if got.err != nil {
		return fmt.Errorf("execute %s: %w", what, got.err)
	}
	if got.code != 0 {
		return fmt.Errorf("%s exit %d: %s: %w", what, got.code, m3Tail(got), ErrAcceptance)
	}
	return nil
}

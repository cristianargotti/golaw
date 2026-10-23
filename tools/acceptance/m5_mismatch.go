package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
)

// The key a bundle of another kit version could carry, the remedy the
// refusal names for it, the subcommand that takes a bundle away and the
// word a digest refusal names.
const (
	m5UnknownKey = "unknown_key"
	m5NewerKit   = "a bundle written for a newer golaw needs that golaw"
	m5RemoveSub  = "remove"
	m5DigestWord = "digest"
)

// m5ScanRefusal is the answer of scan under a broken bundle (D8): exit 2,
// nothing on stdout, and one stderr line starting with golaw: that holds
// every fragment.
func m5ScanRefusal(got result, fragments ...string) error {
	if got.err != nil {
		return fmt.Errorf("execute golaw scan: %w", got.err)
	}
	line := strings.TrimSpace(strings.ReplaceAll(got.stderr, "\r\n", "\n"))
	if got.code != 2 || got.stdout != "" || line == "" || strings.Contains(line, "\n") || !strings.HasPrefix(line, "golaw: ") {
		return fmt.Errorf("scan exited %d with %q and %q, want 2 and one golaw: line: %w", got.code, got.stdout, line, ErrAcceptance)
	}
	for _, fragment := range fragments {
		if !strings.Contains(line, fragment) {
			return fmt.Errorf("the refusal %q does not name %q: %w", line, fragment, ErrAcceptance)
		}
	}
	return nil
}

// m5AppendRules moves the files of the current bundle under its stamp,
// the state of a bundle changed after its install.
func m5AppendRules(dirs policy.Dirs) error {
	path := filepath.Join(profile.Dir(dirs.Config, m5Name), profile.RulesFile)
	body, err := readFile(path)
	if err != nil {
		return err
	}
	return writeFile(path, append(body, []byte("# one byte after the tag\n")...), fileMode)
}

// m5InvalidBundle writes, as the current bundle, the fixture with a key
// this kit does not know, stamped so the stamp, the digest and the pin
// agree and only the schema refuses it.
func m5InvalidBundle(root string, dirs policy.Dirs) (policy.ProfileRef, error) {
	dir := profile.Dir(dirs.Config, m5Name)
	if err := m5CopyBundle(root, dir); err != nil {
		return policy.ProfileRef{}, err
	}
	rules := filepath.Join(dir, profile.RulesFile)
	body, err := readFile(rules)
	if err != nil {
		return policy.ProfileRef{}, err
	}
	if stepErr := writeFile(rules, append(body, []byte(m5UnknownKey+": true\n")...), fileMode); stepErr != nil {
		return policy.ProfileRef{}, stepErr
	}
	return m5Stamp(dir, m5Version, m5Date)
}

// m5PinnedRepository is a committed repository whose golaw.yml pins ref,
// never enrolled: scan still loads the policy of an unenrolled location.
func (s *suite) m5PinnedRepository(ctx context.Context, name string, ref policy.ProfileRef) (string, error) {
	repo, err := s.repository(ctx, name, false)
	if err != nil {
		return "", err
	}
	data, err := bootstrap.Render(bootstrap.Inputs{
		KitVersion: m5CandidateVersion, Language: "pt-BR", Capabilities: []string{"go", "image"},
		Layout: bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/adr", Go: true}, Profile: &ref,
	})
	if err != nil {
		return "", fmt.Errorf("render the pinned policy: %w", err)
	}
	for path, body := range map[string][]byte{moduleFile: []byte(m5Module), policyFile: data} {
		if stepErr := writeFile(filepath.Join(repo, path), body, fileMode); stepErr != nil {
			return "", stepErr
		}
	}
	steps := [][]string{{commandInit, m2Quiet}, {gitAdd, "."}, commitArgs("test: pin a bundle by hand")}
	return repo, s.gitSteps(ctx, repo, steps...)
}

func (s *suite) m5DigestMismatch(ctx context.Context) error {
	setup, err := s.m5CleanRepository(ctx, "digest")
	if err != nil {
		return err
	}
	if stepErr := success(s.m5Golaw(ctx, setup.repo, commandScan, ".")); stepErr != nil {
		return fmt.Errorf("the scan with the bundle as installed: %w", stepErr)
	}
	dirs, err := s.m5Dirs()
	if err != nil {
		return err
	}
	if stepErr := m5AppendRules(dirs); stepErr != nil {
		return stepErr
	}
	return m5ScanRefusal(s.m5Golaw(ctx, setup.repo, commandScan, "."), m5DigestWord, m5InstallCommand)
}

func (s *suite) m5SchemaMismatch(ctx context.Context) error {
	dirs, err := s.m5Dirs()
	if err != nil {
		return err
	}
	ref, err := m5InvalidBundle(s.root, dirs)
	if err != nil {
		return err
	}
	repo, err := s.m5PinnedRepository(ctx, "schema", ref)
	if err != nil {
		return err
	}
	return m5ScanRefusal(s.m5Golaw(ctx, repo, commandScan, "."), profile.RulesFile, m5UnknownKey, m5NewerKit)
}

// m5StaleBundle leaves a current bundle at another version and no stored
// copy of the pin, then installs the pinned version the line names.
func (s *suite) m5StaleBundle(ctx context.Context) error {
	setup, err := s.m5CleanRepository(ctx, "stale")
	if err != nil {
		return err
	}
	if stepErr := success(s.m5Profile(ctx, m5RemoveSub, m5Name)); stepErr != nil {
		return stepErr
	}
	if _, stepErr := s.m5Install(ctx, setup.source, m5NewerTag); stepErr != nil {
		return stepErr
	}
	refusal := s.m5Golaw(ctx, setup.repo, commandScan, ".")
	if stepErr := m5ScanRefusal(refusal, m5Version, m5NewerVersion, m5InstallCommand); stepErr != nil {
		return stepErr
	}
	if _, stepErr := s.m5Install(ctx, setup.source, m5Tag); stepErr != nil {
		return stepErr
	}
	return success(s.m5Golaw(ctx, setup.repo, commandScan, "."))
}

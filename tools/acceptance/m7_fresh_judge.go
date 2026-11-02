package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/genheader"
)

// The base name prefix of every golaw regenerable file, and the make
// targets that run the red tests and the generation check.
const (
	m7RegenerablePrefix = "zz_generated_"
	m7TestTarget        = "test"
	m7CheckTarget       = "generate-check"
)

// m7TargetPattern reads a failed target of GNU make, which prints [target]
// in 3.81 and [file:line: target] in 4.x.
var m7TargetPattern = regexp.MustCompile(`\*{3} \[(?:[^\]]*:\d+: )?([A-Za-z0-9_.-]+)\] Error`)

// m7RedTargets are the make targets that run the red tests or the
// generation check, the only ones allowed to fail before the fills.
func m7RedTargets() []string {
	return []string{m7TestTarget, "coverage-check", "coverage-new", m7CheckTarget}
}

// m7FailedTargets are the make targets output reports as failed, sorted.
func m7FailedTargets(output string) []string {
	matches := m7TargetPattern.FindAllStringSubmatch(output, -1)
	targets := make([]string, 0, len(matches))
	for _, match := range matches {
		targets = append(targets, match[1])
	}
	slices.Sort(targets)
	return slices.Compact(targets)
}

// m7JudgeHolesOpen reads "make ci fails only at the red tests and at golaw
// gen --check" (M7.13 row 1): only the test and check targets fail, the
// red test of every hole fails, and the check names every hole.
func m7JudgeHolesOpen(made, checked result, ids []string) error {
	if made.err != nil {
		return fmt.Errorf("make -k ci did not run: %w", made.err)
	}
	if made.code == 0 {
		return fmt.Errorf("make -k ci passed with the holes open: %w", ErrAcceptance)
	}
	output := made.stdout + "\n" + made.stderr
	targets := m7FailedTargets(output)
	for _, target := range targets {
		if !slices.Contains(m7RedTargets(), target) {
			return fmt.Errorf("make -k ci failed at %s, which runs neither the red tests nor the check: %w", target, ErrAcceptance)
		}
	}
	for _, want := range []string{m7TestTarget, m7CheckTarget} {
		if !slices.Contains(targets, want) {
			return fmt.Errorf("make -k ci did not fail at %s with the holes open: %w", want, ErrAcceptance)
		}
	}
	if checked.err != nil {
		return fmt.Errorf("golaw gen --check did not run: %w", checked.err)
	}
	if checked.code != 1 {
		return fmt.Errorf("golaw gen --check answered exit %d, want 1: %w", checked.code, ErrAcceptance)
	}
	return m7JudgeHoleIDs(output, checked.stdout, ids)
}

// m7JudgeHoleIDs requires every hole named by the check and, unless an
// integration test holds it, its red test failed in the output.
func m7JudgeHoleIDs(output, checked string, ids []string) error {
	for _, id := range ids {
		path, named := m7HolePath(checked, id)
		if !named {
			return fmt.Errorf("golaw gen --check does not name the open hole %s: %w", id, ErrAcceptance)
		}
		// An integration test is built by make test-integration alone, so
		// its hole's red test never runs in make ci; the check names it.
		if !strings.HasSuffix(path, m7IntegrationSuffix) && !strings.Contains(output, "hole "+id+" is open") {
			return fmt.Errorf("the red test of hole %s did not fail: %w", id, ErrAcceptance)
		}
	}
	return nil
}

// m7IntegrationSuffix names the test files make ci never builds.
const m7IntegrationSuffix = "_integration_test.go"

// m7HolePath answers the file of the hole line of contract 2.11 that names
// id, and whether one does.
func m7HolePath(stdout, id string) (string, bool) {
	const prefix = "golaw gen --check: hole open "
	for line := range strings.Lines(stdout) {
		line = strings.TrimRight(line, "\r\n")
		at, ok := strings.CutPrefix(line[max(0, strings.Index(line, prefix)):], prefix)
		if ok && strings.HasSuffix(line, " "+id+": fill it and its red test") {
			file, _, _ := strings.Cut(at, ":")
			return file, true
		}
	}
	return "", false
}

// m7Digest hashes every golaw regenerable file of dir in slash path order,
// so the three native cells of row 6 compare one value.
func m7Digest(dir string) (string, int, error) {
	files, err := m7Regenerable(dir)
	if err != nil {
		return "", 0, err
	}
	if len(files) == 0 {
		return "", 0, fmt.Errorf("no golaw regenerable file under %s: %w", dir, ErrAcceptance)
	}
	paths := slices.Sorted(maps.Keys(files))
	var stream []byte
	for _, path := range paths {
		stream = append(append(stream, path+"\n"...), files[path]...)
	}
	sum := sha256.Sum256(stream)
	return hex.EncodeToString(sum[:]), len(paths), nil
}

// m7Regenerable reads the files of dir golaw regenerates, keyed by slash
// path; one with a carriage return breaks the LF only rule of M7.4.1.
func m7Regenerable(dir string) (map[string][]byte, error) {
	files := make(map[string][]byte)
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		switch {
		case walkErr != nil:
			return walkErr
		case entry.IsDir() && entry.Name() == gitDirectory:
			return filepath.SkipDir
		case !entry.Type().IsRegular() || !strings.HasPrefix(entry.Name(), m7RegenerablePrefix):
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("relate %s to %s: %w", path, dir, err)
		}
		return m7ReadRegenerable(files, path, filepath.ToSlash(rel))
	})
	if err != nil {
		return nil, fmt.Errorf("walk the regenerable files of %s: %w", dir, err)
	}
	return files, nil
}

// m7ReadRegenerable keeps the file at path under rel when its first line is
// the golaw header; a sqlc output named alike carries another header.
func m7ReadRegenerable(files map[string][]byte, path, rel string) error {
	body, err := readFile(path)
	if err != nil {
		return err
	}
	if _, ok := genheader.Parse(bytes.ReplaceAll(body, []byte("\r\n"), []byte("\n"))); !ok {
		return nil
	}
	if bytes.ContainsRune(body, '\r') {
		return fmt.Errorf("the regenerable file %s holds a carriage return: %w", rel, ErrAcceptance)
	}
	files[rel] = body
	return nil
}

func (s *m7Suite) m7DigestCase(ctx context.Context) error {
	repo, err := s.m7FreshRepo(ctx)
	if err != nil {
		return err
	}
	digest, n, err := m7Digest(repo.dir)
	if err != nil {
		return err
	}
	s.m7Note(fmt.Sprintf("rendering digest %s over %d files", digest, n))
	return nil
}

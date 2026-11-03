package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/proof"
)

// The fixture of the M7 rows in the module: the contracts of a fresh
// repository, the recorded outputs of the pinned generators kept apart so
// a row can leave them to setup --gen, and the messages of its commits.
const (
	m7FixtureRoot   = "tools/acceptance/testdata/gen"
	m7RepoDir       = "repo"
	m7ThirdPartyDir = "third-party"
	m7BaseMessage   = "build: adopt golaw over the fixture contracts"
	m7NewMessage    = "feat: write every kind with golaw new"
	m7FillMessage   = "feat: fill every hole"
)

// m7Fixture reads one directory of the M7 fixture, keyed by slash path
// without the escaped suffix, which every file carries so no tool the kit
// runs over itself reads a fixture contract as the kit's own code.

// m7LongPaths lifts the 260 character path limit of Git for Windows in a
// fixture repository: a generated handler test below a test's temporary
// directory passes it, and git add then fails with "Filename too long".
func m7LongPaths() []string { return []string{"config", m2TimingLongPaths, "true"} }

func m7Fixture(root, dir string) (map[string][]byte, error) {
	base := filepath.Join(root, filepath.FromSlash(m7FixtureRoot), dir)
	files := make(map[string][]byte)
	err := fs.WalkDir(os.DirFS(base), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || !entry.Type().IsRegular() {
			return walkErr
		}
		name, escaped := strings.CutSuffix(path, proof.EscapedSuffix)
		if !escaped {
			return fmt.Errorf("the M7 fixture file %s lacks the %s suffix: %w", path, proof.EscapedSuffix, ErrAcceptance)
		}
		body, err := readFile(filepath.Join(base, filepath.FromSlash(path)))
		if err != nil {
			return err
		}
		files[name] = proof.Unescape(body)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read the M7 fixture %s: %w", dir, err)
	}
	return files, nil
}

// m7Command is one golaw new of the fixture: the kind, the name and the
// flags as the command line takes them.
type m7Command struct {
	kind, name string
	flags      []string
}

func (c m7Command) args() []string {
	return append([]string{"new", c.kind, c.name}, c.flags...)
}

// m7Commands write every kind of phase M7.1 once over the fixture. A use
// case creates the ports it names (A56), so each port comes with its use
// case and the second port of ReadPlan is added by the port kind; a port
// that already has its entry is refused, so none is repeated.
func m7Commands() []m7Command {
	usecase := func(name, port, out string) m7Command {
		return m7Command{"usecase", name, []string{"--ports", port, "--in", "AccountID", "--out", out}}
	}
	return []m7Command{
		usecase("growth.ReadPlan", "Ledger", "Plan"),
		{"port", "growth.Clock", []string{"--usecase", "ReadPlan"}},
		usecase("growth.ArchivePlan", "Ledger", "Plan"),
		{"adapter-sql", "postgres.Plans", []string{"--port", "growth.Ledger"}},
		usecase("billing.ReadRate", "Rates", "Rate"),
		{"adapter-http", "rateapi.Client", []string{"--port", "billing.Rates"}},
		{"handler", "growthapi", []string{"--api", "api/openapi.yaml", "--usecases", "growth"}},
	}
}

// m7Repo is an adopted fixture repository and the commit of its adoption,
// the start of the new code period every M7 row measures from.
type m7Repo struct {
	dir, base string
}

// m7Repository writes the fixture contracts, and the recorded generator
// outputs when thirdParty is set, adopts the kit over them and commits the
// adoption.
func (s *m7Suite) m7Repository(ctx context.Context, name string, thirdParty bool) (m7Repo, error) {
	dir := filepath.Join(s.base, dirRepos, name)
	parts := []string{m7RepoDir}
	if thirdParty {
		parts = append(parts, m7ThirdPartyDir)
	}
	for _, part := range parts {
		files, err := m7Fixture(s.root, part)
		if err != nil {
			return m7Repo{}, err
		}
		if err := m6WriteFiles(dir, files); err != nil {
			return m7Repo{}, err
		}
	}
	if err := s.gitSteps(ctx, dir, []string{commandInit, m2Quiet}, m7LongPaths()); err != nil {
		return m7Repo{}, err
	}
	if err := success(s.m5Golaw(ctx, dir, commandInit, m5CapabilitiesFlag, m7Capabilities, flagApply)); err != nil {
		return m7Repo{}, fmt.Errorf("adopt the fixture repository %s: %w", name, err)
	}
	if err := s.m6Commit(ctx, dir, m7BaseMessage); err != nil {
		return m7Repo{}, err
	}
	base, err := s.m7Head(ctx, dir)
	if err != nil {
		return m7Repo{}, err
	}
	return m7Repo{dir: dir, base: base}, nil
}

// m7NewAll runs every command of m7Commands in dir and requires each to
// succeed.
func (s *m7Suite) m7NewAll(ctx context.Context, dir string) error {
	for _, c := range m7Commands() {
		args := c.args()
		if err := success(s.m5Golaw(ctx, dir, args...)); err != nil {
			return fmt.Errorf("golaw %s: %w", strings.Join(args, " "), err)
		}
	}
	return nil
}

// m7Head answers the commit dir is on.
func (s *m7Suite) m7Head(ctx context.Context, dir string) (string, error) {
	got := s.execute(ctx, command{program: s.git, args: []string{m2RevParse, m2Head}, dir: dir, env: s.env})
	if err := success(got); err != nil {
		return "", fmt.Errorf("read the head commit of %s: %w", dir, err)
	}
	return strings.TrimSpace(got.stdout), nil
}

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// m4HomesDir holds the homes of the rows that install apart; codexDir is
// the Codex home under each, where the isolated environment points
// CODEX_HOME.
const (
	m4HomesDir = "m4 homes"
	codexDir   = ".codex"
)

// m4Target is where one M4 replay runs: the home and the environment of
// the host, the repository the payloads name and the golaw binary the
// installed hooks must name.
type m4Target struct {
	home, repo, binary string
	env                []string
	limit              time.Duration // the bound of each shell's replay, the executor's own when zero
}

// m4Shared is the target of the rows that replay through the installation
// the suite prepared.
func (s *suite) m4Shared(repo string) m4Target {
	return m4Target{home: s.home, repo: repo, binary: s.binary, env: s.env}
}

// m4Home makes the home of a row that installs apart and names it by its
// real path: Codex resolves CODEX_HOME through every link and keys hook
// trust by that path, so a home under a linked temporary directory would
// give Codex and the row two keys. The environment is the isolated one
// over that home, with the binary's directory first on PATH.
func (s *suite) m4Home(name string) (string, []string, error) {
	home := filepath.Join(s.base, m4HomesDir, name)
	if err := os.MkdirAll(filepath.Join(home, codexDir), dirMode); err != nil {
		return "", nil, fmt.Errorf("create the home of %s: %w", name, err)
	}
	resolved, err := filepath.EvalSymlinks(home)
	if err != nil {
		return "", nil, fmt.Errorf("resolve the home of %s: %w", name, err)
	}
	return resolved, isolatedEnvironment(os.Environ(), resolved, filepath.Dir(s.binary)), nil
}

// m4Repository is an enrolled repository of a row that installs apart: a
// module file, git init and golaw init --apply run with the target's
// binary in the target's environment, so the enrollment record lands in
// the row's own home.
func (s *suite) m4Repository(ctx context.Context, t m4Target, name string) (string, error) {
	repo := filepath.Join(s.base, dirRepos, name)
	module := []byte("module example.invalid/fixture\n\ngo 1.27\n")
	if err := writeFile(filepath.Join(repo, moduleFile), module, fileMode); err != nil {
		return "", err
	}
	steps := []command{
		{program: s.git, args: []string{commandInit, m2Quiet}, dir: repo, env: t.env},
		{program: t.binary, args: []string{commandInit, flagApply}, dir: repo, env: t.env},
	}
	for _, c := range steps {
		if err := success(s.execute(ctx, c)); err != nil {
			return "", fmt.Errorf("prepare the repository %s: %w", name, err)
		}
	}
	return repo, nil
}

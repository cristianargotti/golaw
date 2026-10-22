package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	scanreport "github.com/cristianargotti/golaw/internal/report"
)

const (
	m3ServiceModule   = "example.test/service"
	m3GoLine          = "1.27"
	m3RulesFile       = "scripts/lint/rules.go"
	m3RuleguardModule = "github.com/quasilyte/go-ruleguard/dsl"
)

// m3ServiceRepository renders the service fixture into a repository of its
// own, adopts the policy under the capabilities, takes the one step init
// proposes and commits everything: the state an adopter reaches before its
// first make ci.
func (s *suite) m3ServiceRepository(ctx context.Context, name string, capabilities []string) (string, error) {
	repo, err := s.repository(ctx, name, false)
	if err != nil {
		return "", err
	}
	if err := m3WriteService(repo); err != nil {
		return "", err
	}
	if err := s.gitSteps(ctx, repo, []string{commandInit, m2Quiet}); err != nil {
		return "", err
	}
	got := s.execute(ctx, command{
		program: s.binary, args: []string{commandInit, flagApply, "--capabilities", strings.Join(capabilities, ",")},
		dir: repo, env: s.env,
	})
	if err := success(got); err != nil {
		return "", err
	}
	if err := s.m3AcceptModule(ctx, repo); err != nil {
		return "", err
	}
	if err := s.gitSteps(ctx, repo, []string{gitAdd, "."}, commitArgs("feat: adopt the golaw policy")); err != nil {
		return "", err
	}
	return repo, nil
}

func m3WriteService(repo string) error {
	files, err := assets.RenderService(golaw.FS, assets.Values{ModulePath: m3ServiceModule, GoVersion: m3GoLine})
	if err != nil {
		return fmt.Errorf("render the service fixture: %w", err)
	}
	for i := range files {
		if err := writeFile(filepath.Join(repo, filepath.FromSlash(files[i].Path)), files[i].Data, fileMode); err != nil {
			return err
		}
	}
	return nil
}

// m3AcceptModule takes the step a person takes after reading the go.mod
// proposal of init: the ruleguard dsl the rendered rules file imports, at
// the version the kit's own go.mod pins, without which make lint cannot
// type-check the rules file. go get records a module only a tagged file
// imports as indirect, and the deadcode target holds go.mod to the shape
// go mod tidy leaves, so tidy follows.
func (s *suite) m3AcceptModule(ctx context.Context, repo string) error {
	if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(m3RulesFile))); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("inspect the rendered rules file: %w", err)
	}
	version, err := m3RequiredVersion(filepath.Join(s.root, moduleFile), m3RuleguardModule)
	if err != nil {
		return err
	}
	for _, args := range [][]string{{"get", m3RuleguardModule + "@" + version}, {"mod", "tidy"}} {
		if err := success(s.execute(ctx, command{program: s.goTool, args: args, dir: repo, env: s.env})); err != nil {
			return err
		}
	}
	return nil
}

func m3RequiredVersion(path, module string) (string, error) {
	body, err := readFile(path)
	if err != nil {
		return "", err
	}
	file, err := modfile.Parse(path, body, nil)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	for _, require := range file.Require {
		if require.Mod.Path == module {
			return require.Mod.Version, nil
		}
	}
	return "", fmt.Errorf("%s does not require %s: %w", path, module, ErrAcceptance)
}

// m3CopyTree copies a repository with its permission bits and refuses any
// entry that is neither a directory nor a regular file, because a link
// would copy something the repository does not hold.
func m3CopyTree(source, target string) error {
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("relativize copied entry: %w", err)
		}
		destination := filepath.Join(target, rel)
		switch {
		case entry.IsDir():
			if err := os.MkdirAll(destination, dirMode); err != nil {
				return fmt.Errorf("create copied directory: %w", err)
			}
			return nil
		case entry.Type().IsRegular():
			return m3CopyFile(path, destination, entry)
		}
		return fmt.Errorf("%s is neither a directory nor a regular file: %w", rel, ErrAcceptance)
	})
	if err != nil {
		return fmt.Errorf("copy the repository tree: %w", err)
	}
	return nil
}

func m3CopyFile(path, destination string, entry fs.DirEntry) error {
	info, err := entry.Info()
	if err != nil {
		return fmt.Errorf("inspect copied file: %w", err)
	}
	body, err := readFile(path)
	if err != nil {
		return err
	}
	return writeFile(destination, body, info.Mode().Perm())
}

func (s *suite) m3Make(ctx context.Context, repo string, args ...string) result {
	return s.execute(ctx, command{program: s.m2.Make, args: args, dir: repo, env: s.env})
}

// m3Scan runs the candidate's scan over a repository and decodes its
// document whatever the exit code, because a red scan is the measurement a
// violation row expects; a document that does not decode names the exit
// and the stderr.
func (s *suite) m3Scan(ctx context.Context, repo string) (scanreport.Document, int, error) {
	got := s.execute(ctx, command{program: s.binary, args: []string{commandScan, m2JSONFlag, "."}, dir: repo, env: s.env})
	if got.err != nil {
		return scanreport.Document{}, 0, fmt.Errorf("execute golaw scan: %w", got.err)
	}
	var doc scanreport.Document
	if err := json.Unmarshal([]byte(got.stdout), &doc); err != nil {
		return scanreport.Document{}, got.code, fmt.Errorf("decode the scan document after exit %d: %s: %w",
			got.code, strings.TrimSpace(got.stderr), err)
	}
	return doc, got.code, nil
}

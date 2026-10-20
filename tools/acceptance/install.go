package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
)

const (
	policyFile    = "golaw.yml"
	binaryName    = "golaw"
	hooksFileName = "hooks.json"
	commandHook   = "hook"
	keyCommand    = "command"
	dirTools      = "tools"
	dirAcceptance = "acceptance"
	dirRepos      = "repositories"
	goBuild       = "build"
	commandInit   = "init"
	commandScan   = "scan"
	flagApply     = "--apply"
	gitAdd        = "add"
	moduleFile    = "go.mod"
)

type suite struct {
	root            string
	base            string
	home            string
	binary          string
	git             string
	goTool          string
	env             []string
	hookFile        string
	hooks           map[string][]hookGroup
	digest          string
	execute         executor
	lookup          locator
	m2              *m2Shared
	m2OS            string
	m2Prerequisites func(*suite, string, string) ([]string, error)
	m2Timings       map[string]command
}

func newSuite(root, base string, execute executor, lookup locator) *suite {
	base = filepath.Join(base, "acceptance área")
	home, bin := filepath.Join(base, "home"), filepath.Join(base, "bin")
	return &suite{
		root: root, base: base, home: home,
		binary:  filepath.Join(bin, executable(binaryName)),
		env:     isolatedEnvironment(os.Environ(), home, bin),
		execute: execute, lookup: lookup,
	}
}

func (s *suite) prepare(ctx context.Context) error {
	if err := s.tools(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.binary), dirMode); err != nil {
		return fmt.Errorf("create binary directory: %w", err)
	}
	got := s.execute(ctx, command{
		program: s.goTool,
		args:    []string{goBuild, "-trimpath", "-o", s.binary, "./cmd/golaw"},
		dir:     s.root, env: s.env,
	})
	if err := success(got); err != nil {
		return fmt.Errorf("build real binary: %w", err)
	}
	if err := s.install(ctx); err != nil {
		return err
	}
	body, err := readFile(s.binary)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(body)
	s.digest = hex.EncodeToString(digest[:])
	return nil
}

func (s *suite) repository(ctx context.Context, name string, enrolled bool) (string, error) {
	repo := filepath.Join(s.base, dirRepos, name)
	if err := os.MkdirAll(repo, dirMode); err != nil {
		return "", fmt.Errorf("create fixture repository: %w", err)
	}
	if !enrolled {
		return repo, nil
	}
	got := s.execute(ctx, command{
		program: s.git, args: []string{commandInit, "--quiet"}, dir: repo, env: s.env,
	})
	if err := success(got); err != nil {
		return "", err
	}
	if err := writeFile(filepath.Join(repo, moduleFile),
		[]byte("module example.invalid/fixture\n\ngo 1.27\n"), fileMode); err != nil {
		return "", err
	}
	got = s.execute(ctx, command{
		program: s.binary, args: []string{commandInit, flagApply}, dir: repo, env: s.env,
	})
	if err := success(got); err != nil {
		return "", err
	}
	return repo, nil
}

func (s *suite) tools() error {
	var err error
	s.goTool, err = s.lookup("go")
	if err != nil {
		return fmt.Errorf("locate Go: %w", err)
	}
	s.git, err = s.lookup("git")
	if err != nil {
		return fmt.Errorf("locate git: %w", err)
	}
	return nil
}

func (s *suite) install(ctx context.Context) error {
	settings := filepath.Join(s.home, ".claude", "settings.json")
	body := `{"permissions":{"deny":["Read(keep-a)","Read(keep-b)"]},"enabledPlugins":{"foreign@local":true}}`
	if err := writeFile(settings, []byte(body), fileMode); err != nil {
		return err
	}
	repo, err := s.repository(ctx, "installation", true)
	if err != nil {
		return err
	}
	c := command{program: s.binary, args: installArgs(), dir: repo, env: s.env}
	if installErr := success(s.execute(ctx, c)); installErr != nil {
		return installErr
	}
	first, err := treeDigests(s.home)
	if err != nil {
		return err
	}
	if againErr := s.reinstall(ctx, c, first); againErr != nil {
		return againErr
	}
	if settingsErr := preservedSettings(settings); settingsErr != nil {
		return settingsErr
	}
	s.hookFile, s.hooks, err = discoverHooks(s.home)
	return err
}

// installArgs keeps every row that installs on the one command the
// adoption sequence documents.
func installArgs() []string {
	return []string{"agent", "install", "--agent", agentClaude}
}

func (s *suite) reinstall(ctx context.Context, c command, first map[string]string) error {
	second := s.execute(ctx, c)
	if err := success(second); err != nil {
		return err
	}
	if strings.Contains(second.stdout, "+++ ") || strings.Contains(second.stdout, "--- ") {
		return fmt.Errorf("second installation printed a diff: %w", ErrAcceptance)
	}
	after, err := treeDigests(s.home)
	if err != nil {
		return err
	}
	if !maps.Equal(first, after) {
		return fmt.Errorf("second installation changed settings, receipts or assets: %w", ErrAcceptance)
	}
	return nil
}

func preservedSettings(path string) error {
	var preserved struct {
		Permissions struct {
			Deny []string `json:"deny"`
		} `json:"permissions"`
		Plugins map[string]bool `json:"enabledPlugins"`
	}
	body, err := readFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, &preserved); err != nil {
		return fmt.Errorf("decode preserved settings: %w", err)
	}
	if len(preserved.Permissions.Deny) < 2 ||
		preserved.Permissions.Deny[0] != "Read(keep-a)" ||
		preserved.Permissions.Deny[1] != "Read(keep-b)" ||
		!preserved.Plugins["foreign@local"] {
		return fmt.Errorf("foreign settings or ordering changed: %w", ErrAcceptance)
	}
	return nil
}

func treeDigests(root string) (map[string]string, error) {
	out := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		body, err := readFile(path)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(body)
		out[path] = hex.EncodeToString(digest[:])
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("digest installed tree: %w", err)
	}
	return out, nil
}

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// The project rows: a Claude installation at project scope writes its
// machine-local entries, which name the marketplace under the person's
// state directory, into .claude/settings.local.json, which git ignores
// through the exclude file of the clone, and an install over a receipt an
// earlier release wrote moves the entries out of the shared settings.json.
const (
	categoryProject   = "project"
	caseProjectLocal  = "claude-local-settings"
	caseProjectMoved  = "claude-shared-moved"
	claudeConfigDir   = ".claude"
	sharedSettings    = "settings.json"
	localSettings     = "settings.local.json"
	projectExclude    = "/.claude/settings.local.json"
	projectPersonKey  = "model"
	projectMovedNote  = "As entradas do golaw saíram de"
	golawPluginID     = "golaw@golaw"
	golawMarketplace  = "golaw"
	gitCheckIgnore    = "check-ignore"
	gitQuietFlag      = "-q"
	projectStatePaths = "state/golaw/claude/project/*/receipt.json"
)

// projectDocument is the part of a Claude settings file the rows judge.
type projectDocument struct {
	Plugins map[string]bool            `json:"enabledPlugins"`
	Markets map[string]json.RawMessage `json:"extraKnownMarketplaces"`
	Model   string                     `json:"model"`
}

// projectRows are the rows of the Claude project installation.
func (s *suite) projectRows(ctx context.Context) []row {
	return []row{
		measured(categoryProject, caseProjectLocal, s.projectLocal(ctx)),
		measured(categoryProject, caseProjectMoved, s.projectMoved(ctx)),
	}
}

// projectLocal installs, reinstalls and uninstalls at project scope: the
// entries land in settings.local.json and never in settings.json, git
// ignores the file, a second install prints no diff, and the uninstall
// takes the file and the exclude line away.
func (s *suite) projectLocal(ctx context.Context) error {
	t, err := s.projectTarget(ctx, caseProjectLocal)
	if err != nil {
		return err
	}
	return runProjectSteps([]func() error{
		func() error { return success(s.projectAgent(ctx, t, verbInstall)) },
		func() error { return projectInstalled(t.repo, false) },
		func() error { return s.projectIgnored(ctx, t) },
		func() error { return projectQuiet(s.projectAgent(ctx, t, verbInstall)) },
		func() error { return success(s.projectAgent(ctx, t, verbUninstall)) },
		func() error { return projectRemoved(t.repo) },
	})
}

// projectMoved rewrites a project installation into the layout of a
// release before settings.local.json, with a key of the person in the
// shared file, and installs again: the install names the move, the shared
// file keeps the person's key alone and the local file holds the entries.
func (s *suite) projectMoved(ctx context.Context) error {
	t, err := s.projectTarget(ctx, caseProjectMoved)
	if err != nil {
		return err
	}
	return runProjectSteps([]func() error{
		func() error { return success(s.projectAgent(ctx, t, verbInstall)) },
		func() error { return projectShare(t) },
		func() error { return answered(s.projectAgent(ctx, t, verbInstall), 0, projectMovedNote) },
		func() error { return projectInstalled(t.repo, true) },
		func() error { return s.projectIgnored(ctx, t) },
	})
}

func runProjectSteps(steps []func() error) error {
	for index, step := range steps {
		if err := step(); err != nil {
			return fmt.Errorf("step %d: %w", index+1, err)
		}
	}
	return nil
}

// projectTarget is an enrolled repository in a home of the row's own, so
// its receipt never touches the installation the other rows read.
func (s *suite) projectTarget(ctx context.Context, name string) (m4Target, error) {
	home, env, err := s.m4Home(name)
	if err != nil {
		return m4Target{}, err
	}
	t := m4Target{home: home, binary: s.binary, env: env}
	t.repo, err = s.m4Repository(ctx, t, categoryProject+"-"+name)
	return t, err
}

// projectAgent runs agent install or uninstall for Claude at project scope.
func (s *suite) projectAgent(ctx context.Context, t m4Target, verb string) result {
	args := []string{commandAgent, verb, flagAgent, agentClaude, flagScope, scopeProject}
	return s.execute(ctx, command{program: t.binary, args: args, dir: t.repo, env: t.env})
}

// projectQuiet holds a repeated install to success without a diff.
func projectQuiet(got result) error {
	if err := success(got); err != nil {
		return err
	}
	if strings.Contains(got.stdout, diffHeader) {
		return fmt.Errorf("a second project install printed a diff: %w", ErrAcceptance)
	}
	return nil
}

// projectIgnored asks git whether it ignores the local settings file.
func (s *suite) projectIgnored(ctx context.Context, t m4Target) error {
	path := claudeConfigDir + "/" + localSettings
	got := s.execute(ctx, command{program: s.git, args: []string{gitCheckIgnore, gitQuietFlag, path}, dir: t.repo, env: t.env})
	if err := success(got); err != nil {
		return fmt.Errorf("git does not ignore %s: %w", path, err)
	}
	return nil
}

// projectInstalled reads both settings files: the local one holds the
// plugin and its marketplace, the shared one holds none of them and, when
// it is kept, the person's key.
func projectInstalled(repo string, sharedKept bool) error {
	var local, shared projectDocument
	if err := projectRead(filepath.Join(repo, claudeConfigDir, localSettings), &local, true); err != nil {
		return err
	}
	if !local.Plugins[golawPluginID] || local.Markets[golawMarketplace] == nil {
		return fmt.Errorf("%s misses the golaw entries: %w", localSettings, ErrAcceptance)
	}
	if err := projectRead(filepath.Join(repo, claudeConfigDir, sharedSettings), &shared, sharedKept); err != nil {
		return err
	}
	if shared.Plugins[golawPluginID] || shared.Markets[golawMarketplace] != nil || sharedKept != (shared.Model != "") {
		return fmt.Errorf("%s holds golaw entries or lost the person's key: %+v: %w", sharedSettings, shared, ErrAcceptance)
	}
	return nil
}

// projectRemoved holds the uninstall to removing the local file it created
// and the exclude line it added, the shared file still absent.
func projectRemoved(repo string) error {
	for _, name := range []string{localSettings, sharedSettings} {
		if exists(filepath.Join(repo, claudeConfigDir, name)) {
			return fmt.Errorf("%s remains after the uninstall: %w", name, ErrAcceptance)
		}
	}
	body, err := readFile(projectExcludeFile(repo))
	if err == nil && bytes.Contains(body, []byte(projectExclude)) {
		return fmt.Errorf("the exclude line remains after the uninstall: %w", ErrAcceptance)
	}
	return nil
}

// projectExcludeFile is the exclude file of the clone at repo.
func projectExcludeFile(repo string) string {
	return filepath.Join(repo, gitDirectory, "info", "exclude")
}

// projectRead decodes a settings file, which must exist when required and
// must not exist otherwise.
func projectRead(path string, into *projectDocument, required bool) error {
	if !required {
		if exists(path) {
			return fmt.Errorf("%s exists: %w", path, ErrAcceptance)
		}
		return nil
	}
	body, err := readFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, into); err != nil {
		return fmt.Errorf(decodeFormat, path, err)
	}
	return nil
}

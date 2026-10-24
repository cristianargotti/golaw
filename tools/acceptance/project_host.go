package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// The host row of the project installation: the validated Claude Code
// loads the plugin a project install declares in settings.local.json once
// the workspace is trusted, and not before. The evidence is the debug log
// of a session that stops at the login, so the row needs no credential.
const (
	caseProjectHost   = "project-local-settings"
	validatedHost     = "2.1.299"
	hostLoadedLine    = "Loading hooks from plugin: golaw"
	hostConfigFile    = ".claude.json"
	hostTrustKey      = "hasTrustDialogAccepted"
	hostPrintFlag     = "-p"
	hostDebugFileFlag = "--debug-file"
)

// projectHostRow judges the host only at the validated version, whose
// debug log names the plugins it loads; any other version is a skip.
func (s *suite) projectHostRow(ctx context.Context, claude, version string) row {
	if !strings.HasPrefix(version, validatedHost+" ") && version != validatedHost {
		return row{
			categoryVendor, caseProjectHost, statusSkip,
			"the load evidence reads the debug log of Claude Code " + validatedHost + ", not " + version, false,
		}
	}
	got := measured(categoryVendor, caseProjectHost, s.projectHost(ctx, claude))
	if got.status == statusPass {
		got.detail = "Claude Code " + validatedHost + " loads the plugin from settings.local.json in a trusted workspace only"
	}
	return got
}

// projectHost installs at project scope, then starts a session before and
// after the workspace is trusted.
func (s *suite) projectHost(ctx context.Context, claude string) error {
	t, err := s.projectTarget(ctx, caseProjectHost)
	if err != nil {
		return err
	}
	if installErr := success(s.projectAgent(ctx, t, verbInstall)); installErr != nil {
		return installErr
	}
	loaded, err := s.projectSession(ctx, t, claude, "before-trust")
	if err == nil && loaded {
		err = fmt.Errorf("the host loaded the plugin before the workspace was trusted: %w", ErrAcceptance)
	}
	if err == nil {
		err = projectTrust(t)
	}
	if err == nil {
		loaded, err = s.projectSession(ctx, t, claude, "after-trust")
	}
	if err == nil && !loaded {
		err = fmt.Errorf("the host did not load the plugin from %s in a trusted workspace: %w", localSettings, ErrAcceptance)
	}
	return err
}

// projectSession starts one print session in the repository, which stops
// at the login of the isolated home, and reports whether its debug log
// loaded the hooks of the golaw plugin.
func (s *suite) projectSession(ctx context.Context, t m4Target, claude, name string) (bool, error) {
	log := filepath.Join(t.home, name+".log")
	args := []string{hostPrintFlag, binaryName, hostDebugFileFlag, log}
	got := s.execute(ctx, command{program: claude, args: args, dir: t.repo, env: t.env})
	if got.err != nil {
		return false, success(got)
	}
	body, err := readFile(log)
	if err != nil {
		return false, err
	}
	return bytes.Contains(body, []byte(hostLoadedLine)), nil
}

// projectTrust records what accepting the trust dialog of the host records
// for the repository, keyed by its real path in both separators.
func projectTrust(t m4Target) error {
	repo, err := filepath.EvalSymlinks(t.repo)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", t.repo, err)
	}
	path := filepath.Join(t.home, claudeConfigDir, hostConfigFile)
	config := map[string]any{}
	if body, readErr := readFile(path); readErr == nil {
		if decodeErr := json.Unmarshal(body, &config); decodeErr != nil {
			return fmt.Errorf(decodeFormat, path, decodeErr)
		}
	}
	projects, ok := config["projects"].(map[string]any)
	if !ok {
		projects = map[string]any{}
	}
	// Claude Code keys a project by its path with forward slashes on
	// Windows; both spellings name the same repository elsewhere.
	for _, key := range []string{repo, filepath.ToSlash(repo)} {
		projects[key] = map[string]any{hostTrustKey: true}
	}
	config["projects"] = projects
	body, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf(encodeFailed, path, err)
	}
	return writeFile(path, body, fileMode)
}

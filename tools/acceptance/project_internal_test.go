package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
)

// projectEnrollment is the enrollment port of the simulated installer.
type projectEnrollment struct{}

func (projectEnrollment) WriteEnrollment(_ context.Context, _, policy, profile string) (model.Enrollment, error) {
	return model.Enrollment{State: model.EnrollmentEnrolled, PolicyDigest: policy, ProfileDigest: profile}, nil
}

// simulatedProject answers the commands of the project rows the way the
// real tools do: git init makes the git directory, git check-ignore reads
// the exclude file, a Claude project install or uninstall runs the
// installer package itself, and a print session of the host writes the
// debug log of a host that loads the plugin in a trusted workspace only.
func simulatedProject(ctx context.Context, t *testing.T, c command) (result, bool) {
	t.Helper()
	switch {
	case filepath.Base(c.program) == "git" && c.args[0] == commandInit:
		if err := os.MkdirAll(filepath.Join(c.dir, ".git"), dirMode); err != nil {
			t.Fatal(err)
		}
		return result{}, true
	case c.args[0] == gitCheckIgnore:
		body, err := readFile(filepath.Join(c.dir, ".git", "info", "exclude"))
		if err != nil || !bytes.Contains(body, []byte(projectExclude)) {
			return result{code: 1}, true
		}
		return result{}, true
	case c.args[0] == hostPrintFlag:
		return simulatedSession(t, c), true
	case c.args[0] == commandAgent && slices.Contains(c.args, scopeProject) && slices.Contains(c.args, agentClaude):
		return simulatedProjectAgent(ctx, t, c), true
	}
	return result{}, false
}

// simulatedProjectAgent runs the installer package for a Claude project
// install or uninstall in the home of the command.
func simulatedProjectAgent(ctx context.Context, t *testing.T, c command) result {
	t.Helper()
	home := environmentValue(c.env, "HOME")
	options := install.Options{
		Root: c.dir, ConfigDir: filepath.Join(home, claudeConfigDir),
		StateDir: filepath.Join(environmentValue(c.env, "XDG_STATE_HOME"), binaryName),
		Scope:    scopeProject, Version: "0.1.0", PolicyDigest: "policy", ReadRules: []string{"Read(notes/*)"},
	}
	var out bytes.Buffer
	plan, err := install.PlanClaude(options)
	if err == nil && c.args[1] == verbInstall {
		err = install.Apply(ctx, plan, &out, projectEnrollment{})
	}
	if err == nil && c.args[1] == verbUninstall {
		plan, err = install.PlanUninstall(plan.ReceiptPath)
		if err == nil {
			err = install.Apply(ctx, plan, &out, nil)
		}
	}
	if err != nil {
		return result{code: 1, stderr: err.Error()}
	}
	return result{stdout: out.String() + strings.Join(plan.Notes, "\n")}
}

// simulatedSession writes the debug log of a session: the plugin loads
// when the local settings enable it and the host config trusts the
// repository by its real path.
func simulatedSession(t *testing.T, c command) result {
	t.Helper()
	index := slices.Index(c.args, hostDebugFileFlag)
	repo, err := filepath.EvalSymlinks(c.dir)
	if err != nil || index < 0 {
		t.Fatalf("session %v in %s: %v", c.args, c.dir, err)
	}
	var config struct {
		Projects map[string]map[string]bool `json:"projects"`
	}
	body, err := readFile(filepath.Join(environmentValue(c.env, "HOME"), claudeConfigDir, hostConfigFile))
	if err == nil {
		err = json.Unmarshal(body, &config)
	}
	log := "[DEBUG] session\n"
	if err == nil && config.Projects[repo][hostTrustKey] && exists(filepath.Join(c.dir, claudeConfigDir, localSettings)) {
		log += hostLoadedLine + "\n"
	}
	checkedWrite(t, c.args[index+1], log)
	return result{code: 1, stdout: "Not logged in\n"}
}

func TestProjectRows(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	rows := s.projectRows(t.Context())
	if len(rows) != 2 {
		t.Fatalf("%+v", rows)
	}
	for _, got := range rows {
		assertPass(t, got)
	}
	base := s.execute
	s.execute = func(ctx context.Context, c command) result {
		got := base(ctx, c)
		if len(c.args) > 1 && c.args[0] == commandAgent && c.args[1] == verbInstall {
			local := filepath.Join(c.dir, claudeConfigDir, localSettings)
			if body, err := readFile(local); err == nil {
				checkedWrite(t, filepath.Join(c.dir, claudeConfigDir, sharedSettings), string(body))
			}
		}
		return got
	}
	for _, got := range s.projectRows(t.Context()) {
		if got.status != statusFail {
			t.Errorf("an install that writes the shared settings passed: %+v", got)
		}
	}
}

func TestProjectRowsRefuseBrokenSteps(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	base := s.execute
	s.execute = func(ctx context.Context, c command) result {
		if c.args[0] == gitCheckIgnore {
			return result{code: 1}
		}
		return base(ctx, c)
	}
	if got := s.measureProjectLocal(t); got.status != statusFail || !strings.Contains(got.detail, "ignore") {
		t.Fatalf("an unignored settings file passed: %+v", got)
	}
	s.execute = func(ctx context.Context, c command) result {
		if c.args[0] == commandAgent && c.args[1] == verbUninstall {
			return result{}
		}
		return base(ctx, c)
	}
	if got := s.measureProjectLocal(t); got.status != statusFail || !strings.Contains(got.detail, "remains") {
		t.Fatalf("an uninstall that left the settings passed: %+v", got)
	}
	if err := projectShare(m4Target{home: t.TempDir(), repo: t.TempDir()}); err == nil {
		t.Fatal("a home without a receipt was shared")
	}
	if err := projectRead(filepath.Join(t.TempDir(), "absent.json"), &projectDocument{}, true); err == nil {
		t.Fatal("an absent required settings file was read")
	}
}

func (s *suite) measureProjectLocal(t *testing.T) row {
	t.Helper()
	return measured(categoryProject, caseProjectLocal, s.projectLocal(t.Context()))
}

func TestProjectHostRow(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	got := s.projectHostRow(t.Context(), agentClaude, "2.1.299 (Claude Code)")
	assertPass(t, got)
	if !strings.Contains(got.detail, localSettings) {
		t.Fatalf("%+v", got)
	}
	if got := s.projectHostRow(t.Context(), agentClaude, "2.1.300 (Claude Code)"); got.status != statusSkip || got.required {
		t.Fatalf("another host version must be an optional skip: %+v", got)
	}
	base := s.execute
	s.execute = func(ctx context.Context, c command) result {
		got := base(ctx, c)
		if c.args[0] == hostPrintFlag {
			checkedWrite(t, c.args[slices.Index(c.args, hostDebugFileFlag)+1], hostLoadedLine+"\n")
		}
		return got
	}
	if got := s.projectHostRow(t.Context(), agentClaude, validatedHost); got.status != statusFail {
		t.Fatalf("a host that loads before trust passed: %+v", got)
	}
	s.execute = func(ctx context.Context, c command) result {
		if c.args[0] == hostPrintFlag {
			return result{}
		}
		return base(ctx, c)
	}
	if got := s.projectHostRow(t.Context(), agentClaude, validatedHost); got.status != statusFail {
		t.Fatalf("a host that never loads passed: %+v", got)
	}
	s.execute = func(ctx context.Context, c command) result {
		if c.args[0] == hostPrintFlag {
			return result{err: ErrAcceptance}
		}
		return base(ctx, c)
	}
	if got := s.projectHostRow(t.Context(), agentClaude, validatedHost); got.status != statusFail {
		t.Fatalf("a host that cannot start passed: %+v", got)
	}
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

func payload(root, repo string, f fixture) ([]byte, error) {
	if f.mode == modeRaw {
		return []byte(f.raw), nil
	}
	body, err := readFile(filepath.Join(root, "tools", "acceptance", "testdata", "claude.json"))
	if err != nil {
		return nil, err
	}
	var value map[string]any
	if decodeErr := json.Unmarshal(body, &value); decodeErr != nil {
		return nil, fmt.Errorf("decode base fixture: %w", decodeErr)
	}
	value["cwd"], value["tool_use_id"] = repo, f.name
	value["hook_event_name"], value["tool_name"], value["tool_input"] = f.event, f.tool, f.input
	switch f.event {
	case eventPost:
		value["tool_response"] = map[string]any{
			"stdout": "", "stderr": "",
			"bashEditDiff": map[string]any{
				"changedFiles": []string{filepath.Join(repo, pathNote)},
			},
		}
	case eventFailure:
		value["error"] = "command failed"
	}
	body, err = json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode fixture: %w", err)
	}
	return body, nil
}

func (s *suite) runFixture(ctx context.Context, f fixture) row {
	repo, err := s.repository(ctx, f.name, f.mode != modeUnenrolled)
	if err == nil {
		err = materialize(repo, f.before)
	}
	if err == nil {
		err = s.configureFixture(ctx, repo, f)
	}
	if err == nil && f.mode == modeShellPost {
		pre := f
		pre.event = eventPre
		err = s.judgeFixture(ctx, repo, pre, wantAllow)
		if err == nil {
			err = materialize(repo, f.after)
		}
	}
	if err == nil {
		err = s.judgeFixture(ctx, repo, f, f.want)
	}
	return measured(f.category, f.name, err)
}

func (s *suite) judgeFixture(ctx context.Context, repo string, f fixture, want string) error {
	body, err := payload(s.root, repo, f)
	if err != nil {
		return err
	}
	return expect(s.invoke(ctx, repo, f.event, f.tool, body), want)
}

func (s *suite) configureFixture(ctx context.Context, repo string, f fixture) error {
	switch f.mode {
	case modeUnreadable:
		if err := os.Mkdir(filepath.Join(repo, "target"), dirMode); err != nil {
			return fmt.Errorf("create unreadable target: %w", err)
		}
	case modeDeleted:
		if err := os.Remove(filepath.Join(repo, policyFile)); err != nil {
			return fmt.Errorf("delete enrolled policy: %w", err)
		}
	case modeWeakening:
		return s.weakenPolicy(ctx, repo)
	case modeCommitted, modeUnstaged:
		return s.commitPolicy(ctx, repo, f.mode == modeUnstaged)
	}
	return nil
}

// commitPolicy gives the repository a first commit that holds the policy,
// so HEAD governs instead of the bootstrap record; unstage then drops the
// file from the index and keeps it in the checkout, the deletion the next
// commit would carry.
func (s *suite) commitPolicy(ctx context.Context, repo string, unstage bool) error {
	steps := [][]string{{gitAdd, "--", policyFile}, commitArgs("build: adopt the policy")}
	if unstage {
		steps = append(steps, []string{"rm", "-q", "--cached", "--", policyFile})
	}
	return s.gitSteps(ctx, repo, steps...)
}

// commitArgs is a commit in the isolated home, which carries no git
// identity and no signing key of its own.
func commitArgs(message string) []string {
	return []string{
		"-c", "user.name=golaw acceptance", "-c", "user.email=acceptance@example.invalid",
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", message,
	}
}

// gitSteps stops at the first failing command, so no later step runs over
// a repository an earlier one left half made.
func (s *suite) gitSteps(ctx context.Context, repo string, steps ...[]string) error {
	for _, args := range steps {
		if err := success(s.execute(ctx, command{program: s.git, args: args, dir: repo, env: s.env})); err != nil {
			return err
		}
	}
	return nil
}

func (s *suite) weakenPolicy(ctx context.Context, repo string) error {
	path := filepath.Join(repo, policyFile)
	body, err := readFile(path)
	if err != nil {
		return err
	}
	pattern := regexp.MustCompile(`file_lines:\s*300`)
	if !pattern.Match(body) {
		return fmt.Errorf("generated policy lacks file_lines: %w", ErrAcceptance)
	}
	if err := writeFile(path, pattern.ReplaceAll(body, []byte("file_lines: 301")), fileMode); err != nil {
		return err
	}
	return success(s.execute(ctx, command{
		program: s.git, args: []string{gitAdd, "--", policyFile}, dir: repo, env: s.env,
	}))
}

func materialize(repo string, files map[string]string) error {
	for path, content := range files {
		if err := writeFile(filepath.Join(repo, path), []byte(content), fileMode); err != nil {
			return err
		}
	}
	return nil
}

func aliasRow(ctx context.Context, s *suite) row {
	repo, err := s.repository(ctx, caseAlias, true)
	if err != nil {
		return measured(categoryPath, caseAlias, err)
	}
	if err := writeFile(filepath.Join(repo, "AliasProbe"), []byte(textSafe), fileMode); err != nil {
		return measured(categoryPath, caseAlias, err)
	}
	if _, err := os.Stat(filepath.Join(repo, "aliasprobe")); err != nil {
		if os.IsNotExist(err) {
			return row{categoryPath, caseAlias, statusSkip, "filesystem is case sensitive", false}
		}
		return measured(categoryPath, caseAlias, fmt.Errorf("inspect alias: %w", err))
	}
	f := writeFixture(caseAlias, "IMAGE.PNG", textSafe, wantDeny)
	return measured(categoryPath, caseAlias, s.judgeFixture(ctx, repo, f, f.want))
}

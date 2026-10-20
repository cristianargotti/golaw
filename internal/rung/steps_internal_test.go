package rung

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
)

func TestSecretsCommandReadsTheIndexGitCommits(t *testing.T) {
	t.Parallel()
	env := runner.UserEnv([]string{"PATH=/bin", "GIT_DIR=/work/.git", "GIT_INDEX_FILE=.git/index.lock"})
	c := secretsCommand("/tools/gitleaks", "/work", env, "/work/.git/index.lock")
	if !slices.Contains(c.Env, "GIT_INDEX_FILE=/work/.git/index.lock") || slices.Contains(c.Env, "GIT_DIR=/work/.git") || c.Dir != "/work" {
		t.Errorf("command = %+v", c)
	}
	if plain := secretsCommand("/tools/gitleaks", "/work", env, ""); !slices.Equal(plain.Env, []string{"PATH=/bin"}) || len(env) != 1 {
		t.Errorf("without an index = %+v, given %v", plain, env)
	}
}

func TestToolLinesKeepTheFirstTwoHundred(t *testing.T) {
	t.Parallel()
	lines := toolLines(runner.Output{Stdout: []byte(strings.Repeat("issue\n", 250)), Stderr: []byte("done\n")})
	if len(lines) != 201 || lines[0] != "issue" || lines[200] != "... and 51 more lines" {
		t.Errorf("lines = %d, last %q", len(lines), lines[len(lines)-1])
	}
	if lines := toolLines(runner.Output{}); lines != nil {
		t.Errorf("silence = %v", lines)
	}
}

func TestVerifiedKnowsOnlyThePinnedTools(t *testing.T) {
	t.Parallel()
	if _, err := verified(t.Context(), "make", nil); !errors.Is(err, model.ErrUsage) {
		t.Errorf("make = %v", err)
	}
}

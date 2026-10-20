package runner_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
)

func TestHookVariablesNameWhatGitGivesAHook(t *testing.T) {
	t.Parallel()
	want := []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_PREFIX"}
	if got := runner.HookVariables(); !slices.Equal(got, want) {
		t.Errorf("HookVariables() = %v, want %v", got, want)
	}
}

func TestUserEnvStripsTheHookVariablesWhateverTheirCase(t *testing.T) {
	t.Parallel()
	environ := []string{
		"PATH=/bin", "GIT_DIR=/repo/.git", "git_index_file=/repo/.git/index.lock", "GIT_WORK_TREE=/repo",
		"GIT_COMMON_DIR=/repo/.git", "GIT_PREFIX=sub/", "GIT_EDITOR=:", "GIT_DIRECTORY=kept", "LANG=C.UTF-8",
	}
	given := slices.Clone(environ)
	want := []string{"PATH=/bin", "GIT_EDITOR=:", "GIT_DIRECTORY=kept", "LANG=C.UTF-8"}
	if got := runner.UserEnv(environ); !slices.Equal(got, want) {
		t.Errorf("UserEnv = %v, want %v", got, want)
	}
	if !slices.Equal(environ, given) {
		t.Error("UserEnv changed the environment it was given")
	}
	if got := runner.UserEnv(nil); len(got) != 0 {
		t.Errorf("UserEnv(nil) = %v", got)
	}
}

func TestAUserCommandAnswersForItsOwnRepository(t *testing.T) {
	t.Parallel()
	hooked, other := gittest.New(t), gittest.New(t)
	environ := append(gittest.Isolated(), "GIT_DIR="+hooked.GitDir())
	ask := func(env []string) string {
		out, err := runner.Run(t.Context(), runner.Command{Name: "git", Args: []string{"rev-parse", "--absolute-git-dir"}, Dir: other.Root, Env: env})
		if err != nil || out.Code != 0 {
			t.Fatalf("git rev-parse: %+v, %v", out, err)
		}
		return string(out.Stdout)
	}
	if got := ask(runner.UserEnv(environ)); got != other.GitDir()+"\n" {
		t.Errorf("a user command answered for %q, want its own %s", got, other.GitDir())
	}
	if got := ask(environ); got != hooked.GitDir()+"\n" {
		t.Errorf("the plumbing environment keeps the hook's repository: %q", got)
	}
}

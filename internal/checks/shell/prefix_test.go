package shell_test

import (
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/shell"
)

func TestPrefixesAreContinuationSafe(t *testing.T) {
	t.Parallel()
	const reason = "the command skips the git hooks; the hooks are the rule"
	want := []shell.Prefix{
		{
			Check:  "shell.no-verify",
			Words:  [][]string{{"git"}, {"commit"}, {"--no-verify", "-n"}},
			Reason: reason,
		},
		{
			Check:  "shell.no-verify",
			Words:  [][]string{{"git"}, {"merge", "rebase", "push"}, {"--no-verify"}},
			Reason: reason,
		},
	}
	got := shell.Prefixes()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("prefixes = %#v, want %#v", got, want)
	}
	got[0].Words[0][0] = "changed"
	if !reflect.DeepEqual(shell.Prefixes(), want) {
		t.Fatal("caller changed the shared pattern source")
	}
	for _, command := range []string{
		"git commit --no-verify",
		"git commit -n",
		"git merge --no-verify",
		"git rebase --no-verify",
		"git push --no-verify",
	} {
		for _, suffix := range []string{"", " --", " -m x", " --help"} {
			result := judge(t, shell.NoVerify(), command+suffix)
			if len(result.Findings) != 1 ||
				result.Findings[0].Message != reason {
				t.Errorf("%q: %+v", command+suffix, result)
			}
		}
	}
}

func TestPrefixSourceKeepsHookBoundaries(t *testing.T) {
	t.Parallel()
	denies(t, shell.NoVerify(), "no-verify", map[string]bool{
		"git commit -m --no-verify":        false,
		"git commit -- --no-verify":        false,
		"git merge -n topic":               false,
		"git -C project commit -n":         true,
		"bash -lc 'git commit -n'":         true,
		"git commit -m 'fix: safe'":        false,
		"git commit --amend -m x -n":       true,
		"git commit --message=--no-verify": false,
	})
	denies(t, shell.CredentialPrint(), "credential-print", map[string]bool{
		"gh auth token":                         true,
		"gh auth token --hostname example.test": false,
		"printenv":                              true,
		"printenv PATH":                         false,
		"env":                                   true,
		"env command":                           false,
		"aws configure get":                     false,
		"op read":                               false,
		"vault read":                            false,
		"vault kv get":                          false,
	})
}

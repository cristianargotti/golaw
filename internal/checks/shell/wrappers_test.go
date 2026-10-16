package shell_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/shell"
)

// TestEvalAndXargsCarryACommandLine reads the command line eval and xargs
// carry: eval joins its words with spaces and the shell reads the result
// again, quotes and lists included, behind the reserved words and wrappers
// too, and xargs runs the command after its options, whose values (-a, -d,
// -E, -I, -J, -L, -n, -P, -R, -S, -s, the long ones with or without =)
// it reads first, an optional value only attached and -- ending them.
func TestEvalAndXargsCarryACommandLine(t *testing.T) {
	t.Parallel()
	expectCodes(t, shell.NoVerify(), []toolCase{
		{"Bash", "eval 'git commit --no-verify -m x'", "no-verify"},
		{"Bash", "eval git commit --no-verify -m x", "no-verify"},
		{"Bash", `eval "git commit -m x; git commit --no-verify -m y"`, "no-verify"},
		{"Bash", "if true; then eval 'git commit --no-verify -m x'; fi", "no-verify"},
		{"Bash", "command eval 'git commit --no-verify -m x'", "no-verify"},
		{"Bash", `eval "builtin eval 'git commit --no-verify -m x'"`, "no-verify"},
		{"Bash", "echo | xargs git commit --no-verify -m x", "no-verify"},
		{"Bash", "xargs -0 -n 1 git commit --no-verify -m x", "no-verify"},
		{"Bash", "xargs -I {} git commit --no-verify -m {}", "no-verify"},
		{"Bash", "xargs -a list.txt git commit --no-verify -m x", "no-verify"},
		{"Bash", "xargs --max-args=1 git commit --no-verify -m x", "no-verify"},
		{"Bash", "xargs --max-args 1 --null git commit --no-verify -m x", "no-verify"},
		{"Bash", "xargs -L1 git commit --no-verify -m x", "no-verify"},
		{"Bash", "xargs -0tn 1 git commit --no-verify -m x", "no-verify"},
		{"Bash", "xargs -i git commit --no-verify -m {}", "no-verify"},
		{"Bash", "xargs --replace git commit --no-verify -m {}", "no-verify"},
		{"Bash", "xargs -- git commit --no-verify -m x", "no-verify"},
		{"Bash", "xargs sh -c 'git commit --no-verify -m x'", "no-verify"},
		{"Bash", "eval 'git commit -m x'", ""},
		{"Bash", `eval 'git commit -m "--no-verify"'`, ""},
		{"Bash", "echo eval git commit --no-verify", ""},
		{"Bash", "xargs git commit -m x", ""},
		{"Bash", "echo xargs git commit --no-verify", ""},
		{"Bash", "xargs -n", ""},
		{"Bash", "xargs", ""},
		{"Bash", "eval", ""},
	})
}

// TestNoVerifyReadsTheGlobalOptionsOfGit reads the global options of git
// the way git 2.44 does: -C, -c, --git-dir, --work-tree, --namespace,
// --config-env, --attr-source and --shallow-file take the next word as
// their value unless = joins it, so that word is never the subcommand.
func TestNoVerifyReadsTheGlobalOptionsOfGit(t *testing.T) {
	t.Parallel()
	expectCodes(t, shell.NoVerify(), []toolCase{
		{"Bash", "git --git-dir .git commit --no-verify -m x", "no-verify"},
		{"Bash", "git --work-tree . commit --no-verify -m x", "no-verify"},
		{"Bash", "git --namespace x --attr-source HEAD commit --no-verify -m x", "no-verify"},
		{"Bash", "git --config-env core.pager=HOME --shallow-file x commit -n -m x", "no-verify"},
		{"PowerShell", "git --git-dir .git --work-tree . commit --no-verify -m x", "no-verify"},
		{"Bash", "git --git-dir=.git commit --no-verify -m x", "no-verify"},
		{"Bash", "git --config-env core.hooksPath=HOOKS commit -m x", "no-verify"},
		{"Bash", "git --git-dir .git commit -m x", ""},
		{"Bash", "git --work-tree . status", ""},
	})
}

// TestEvalAndXargsKeepTheRolesOfTheirCommand judges the command eval and
// xargs run by its own roles: a read of secret material eval runs is a
// read, and a package pattern of the go tool behind xargs stays a package.
func TestEvalAndXargsKeepTheRolesOfTheirCommand(t *testing.T) {
	t.Parallel()
	expectCodes(t, shell.SecretTokens(), []toolCase{
		{"Bash", "eval 'cat deploy/secrets/token.txt'", "secret-token"},
		{"Bash", "xargs -0 cat deploy/secrets/token.txt", "secret-token"},
		{"Bash", "xargs go vet ./internal/secrets/...", ""},
		{"Bash", "xargs -P 4 go test ./internal/secrets/...", ""},
	})
}

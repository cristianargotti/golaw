package shell

import "slices"

// The id of shell.no-verify and the reason it gives, shared by the check
// and by every prefix the table below yields, so a rules file a host
// generates from the table justifies a denial with the hook's own words.
const (
	noVerifyID     = "shell.no-verify"
	noVerifyReason = "the command skips the git hooks; the hooks are the rule"
	programGit     = "git"
)

// Prefix is one command prefix a check denies whatever words follow it:
// the id of the check, one element per argument position with the
// alternatives of that position, and the English reason of the check. A
// host that forbids commands by their leading words reads this table, the
// same one the hook judges with, so the two never disagree.
type Prefix struct {
	Check  string
	Words  [][]string
	Reason string
}

// Prefixes lists the prefixes in check order. Only a prefix the hook
// denies for every continuation is listed: an option the hook reads
// anywhere in the command, a directory option such as git -C and a
// wrapper such as bash -lc have no prefix form and stay the hook's alone,
// and a family whose verdict depends on the policy is never listed. Every
// call answers fresh slices, so a caller never edits the shared source.
func Prefixes() []Prefix {
	return []Prefix{
		{
			Check:  noVerifyID,
			Words:  [][]string{{programGit}, {subCommit}, {noVerifyFlag, "-n"}},
			Reason: noVerifyReason,
		},
		{
			Check:  noVerifyID,
			Words:  [][]string{{programGit}, {"merge", "rebase", subPush}, {noVerifyFlag}},
			Reason: noVerifyReason,
		},
	}
}

// bypassWords answers the flags that skip the hooks on a git subcommand,
// read from the prefix table so the hook denies exactly what the table
// forbids; a subcommand the table does not name has none.
func bypassWords(subcommand string) []string {
	for _, prefix := range Prefixes() {
		words := prefix.Words
		if prefix.Check == noVerifyID && len(words) > 2 && slices.Contains(words[1], subcommand) {
			return words[len(words)-1]
		}
	}
	return nil
}

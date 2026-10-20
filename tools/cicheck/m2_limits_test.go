package cicheck_test

import "testing"

// TestLefthookQuoteLimitIsStated pins the stated limit of the quoted
// arguments the lefthook.yml asset hands golaw hook: lefthook splices them
// into sh -c unescaped, so an argument that holds a single quote fails the
// hook before golaw runs, where no ladder switch reaches it.
func TestLefthookQuoteLimitIsStated(t *testing.T) {
	t.Parallel()
	requireText(t, "docs/security.md",
		"An argument that holds a single quote is a stated limit of the lefthook.yml asset",
		"the absolute message path Git gives a commit in a linked worktree of a repository whose path holds an apostrophe, or a remote name or URL that holds one, leaves the quote open",
		"so the hook fails with a shell syntax error before golaw runs and no ladder switch reaches it",
		"commit from the main worktree, push through a remote whose name and URL hold no apostrophe, or run golaw setup --uninstall in that clone",
	)
	requireText(t, "CHANGELOG.md",
		"an argument that holds a single quote, such as the message file of a linked worktree of a repository whose path holds an apostrophe or a remote URL with one, still breaks lefthook's quoting, so the hook fails before golaw runs, a stated limit in docs/security.md.",
	)
}

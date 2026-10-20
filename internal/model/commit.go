package model

import (
	"fmt"
	"strings"
)

// The cleanup modes of git's commit.cleanup, which git applies to the
// message after the commit-msg hook has read it.
const (
	CleanupStrip      = "strip"
	CleanupWhitespace = "whitespace"
	CleanupVerbatim   = "verbatim"
	CleanupScissors   = "scissors"
)

// DefaultCommentChar is the comment character git uses when
// core.commentChar is unset.
const DefaultCommentChar = "#"

// Commit is the commit a commit check judges in commit mode: the message as
// git gives it to the commit-msg hook, before its own cleanup, what that
// cleanup needs, and the identity the commit is authored under.
type Commit struct {
	Message     string `json:"message"`                // the message file as git wrote it
	Cleanup     string `json:"cleanup,omitempty"`      // one of the Cleanup modes; empty is CleanupStrip
	CommentChar string `json:"comment_char,omitempty"` // core.commentChar or core.commentString; empty is DefaultCommentChar
	Author      string `json:"author,omitempty"`       // "Name <email>" as git var GIT_AUTHOR_IDENT names it, empty when unknown
}

// Validate refuses a cleanup mode git never applies after the hook, the
// configured "default" included, which the rung resolves before it builds
// the commit, and a comment string git refuses: one that holds a line
// break. Git takes any other, a space, a tab or a carriage return included,
// so refusing more would fail a commit git makes. A rung bug is then a
// usage error, never a message judged under the wrong rules.
func (c *Commit) Validate() error {
	switch c.Cleanup {
	case "", CleanupStrip, CleanupWhitespace, CleanupVerbatim, CleanupScissors:
	default:
		return fmt.Errorf("%w: commit cleanup %q is not strip, whitespace, verbatim or scissors", ErrUsage, c.Cleanup)
	}
	if strings.Contains(c.CommentChar, "\n") {
		return fmt.Errorf("%w: commit comment string %q", ErrUsage, c.CommentChar)
	}
	return nil
}

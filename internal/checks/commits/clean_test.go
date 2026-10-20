package commits_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/commits"
	"github.com/cristianargotti/golaw/internal/model"
)

// cut is the scissors line git writes after the comment string and a space.
const cut = " ------------------------ >8 ------------------------\n"

func TestCleanAppliesTheCleanupGitApplies(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		commit model.Commit
		want   string
	}{
		"strip drops comments and spaces": {model.Commit{Message: "\n  \nfix: x  \n\n# a comment\nbody\t\n\n\n\nmore\n\n"}, "fix: x\n\nbody\n\nmore\n"},
		"strip by its name":               {model.Commit{Message: "fix: x\n# gone\n", Cleanup: model.CleanupStrip}, "fix: x\n"},
		"a comment between blank lines":   {model.Commit{Message: "fix: x\n\n# c\n\nbody\n"}, "fix: x\n\nbody\n"},
		"whitespace keeps comments":       {model.Commit{Message: "fix: x  \n# kept\n\n\n", Cleanup: model.CleanupWhitespace}, "fix: x\n# kept\n"},
		"verbatim keeps every byte":       {model.Commit{Message: "  fix: x  \n\n\n# kept", Cleanup: model.CleanupVerbatim}, "  fix: x  \n\n\n# kept"},
		"scissors keeps comments":         {model.Commit{Message: "fix: x\n# kept\n#" + cut + "diff\n", Cleanup: model.CleanupScissors}, "fix: x\n# kept\n"},
		"verbose diff under strip":        {model.Commit{Message: "fix: x\n\n#" + cut + "diff --git a/a b/a\n+x\n"}, "fix: x\n"},
		"verbose diff under verbatim":     {model.Commit{Message: "fix: x\n#" + cut + "diff\n", Cleanup: model.CleanupVerbatim}, "fix: x\n"},
		"scissors on the first line":      {model.Commit{Message: "#" + cut + "fix: x\n"}, ""},
		"scissors of another comment":     {model.Commit{Message: "fix: x\n;" + cut, Cleanup: model.CleanupWhitespace}, "fix: x\n;" + cut},
		"scissors not at a line start":    {model.Commit{Message: "fix: x #" + cut, Cleanup: model.CleanupWhitespace}, "fix: x #" + cut},
		"a configured comment character":  {model.Commit{Message: "fix: x\n; note\n# kept\n;" + cut + "below\n", CommentChar: ";"}, "fix: x\n# kept\n"},
		"a comment string":                {model.Commit{Message: "fix: x\n// note\n", CommentChar: "//"}, "fix: x\n"},
		"carriage returns":                {model.Commit{Message: "fix: x\r\n\r\nbody\r\n"}, "fix: x\n\nbody\n"},
		"git keeps a vertical tab":        {model.Commit{Message: "fix: x\v\n"}, "fix: x\v\n"},
		"an empty message":                {model.Commit{}, ""},
		"only comments":                   {model.Commit{Message: "# one\n# two\n"}, ""},
	}
	for name, c := range cases {
		if got := commits.Clean(c.commit); got != c.want {
			t.Errorf("%s: Clean = %q, want %q", name, got, c.want)
		}
	}
}

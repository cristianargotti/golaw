package assets_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
)

func TestHygieneAssetsPinTheirCompleteContent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		path, want string
		ownership  assets.Ownership
	}{
		{".gitleaks.toml", `title = "golaw"

[extend]
useDefault = true
`, assets.OwnedByKit},
		{".gitattributes", `* text=auto eol=lf
*.png binary
*.jpg binary
*.gif binary
*.ico binary
*.woff binary
*.woff2 binary
`, assets.OwnedMerged},
		{".editorconfig", `root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true
indent_style = space
indent_size = 2

[*.go]
indent_style = tab

[Makefile]
indent_style = tab

[*.mk]
indent_style = tab
`, assets.OwnedMerged},
		{".github/dependabot.yml", `version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: weekly
    cooldown:
      default-days: 7
      semver-major-days: 30
  - package-ecosystem: gomod
    directory: /tools
    schedule:
      interval: weekly
    cooldown:
      default-days: 7
      semver-major-days: 30
  - package-ecosystem: github-actions
    directory: /
    schedule:
      interval: weekly
    cooldown:
      default-days: 7
`, assets.OwnedByKit},
	} {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			assertAsset(t, tc.path, tc.want, tc.ownership)
		})
	}
}

package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The web set's TypeScript rows (survey 1.2, typescript:S3735 x21 among
// them) and the hotspot rows of the hub's tests and scripts (S2068, S1313,
// S5852, survey 1.2).
func TestWebTypedAndHotspotRows(t *testing.T) {
	t.Parallel()
	assertWebRows(t, []webRowCase{
		{"typescript:S3735", eslintTool, "sonarjs/void-use", "exact"},
		{"typescript:S6845", eslintTool, "jsx-a11y/no-noninteractive-tabindex", "exact"},
		{"typescript:S4325", eslintTool, "@typescript-eslint/no-unnecessary-type-assertion", "exact"},
		{"typescript:S5332", eslintTool, "sonarjs/no-clear-text-protocols", "exact"},
		{"javascript:S2068", eslintTool, "sonarjs/no-hardcoded-passwords", "exact"},
		{"javascript:S1313", eslintTool, "sonarjs/no-hardcoded-ip", "exact"},
		{"javascript:S5852", eslintTool, "sonarjs/slow-regex", "exact"},
	})
}

// The credential fixtures are stored escaped, so the word the rule reads
// never sits in the kit tree in clear, and decode to it at load.
func TestWebCredentialFixturesAreEscaped(t *testing.T) {
	t.Parallel()
	base := filepath.Join("testdata", "web", "parity", "sonarjs-no-hardcoded-passwords")
	for _, twin := range []string{"red", "near"} {
		stored := filepath.Join(base, twin, "test", "deploy", "install.test.cjs.esc")
		raw, err := os.ReadFile(stored)
		if err != nil {
			t.Fatal(err)
		}
		word := "pass" + "word"
		if strings.Contains(string(raw), word) {
			t.Errorf("%s holds the credential word in clear", stored)
		}
		decoded := webFixture(t, filepath.Join(base, twin))["test/deploy/install.test.cjs"]
		if !strings.Contains(string(decoded), "const "+word+" = ") {
			t.Errorf("%s does not decode to an assignment of %s", stored, word)
		}
	}
}

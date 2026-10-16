package hook

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

// TestWindowsDriveReadsTheDriveFormOfTheHost reads a name the way the host
// reads it on Windows before it opens it: a slash, one letter and a slash
// name that drive, and any other name is left as it is.
func TestWindowsDriveReadsTheDriveFormOfTheHost(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]string{
		"/c/work/golaw.yml": `C:\work\golaw.yml`,
		"/D/":               `D:\`,
		"/c":                "/c",
		"/cc/work":          "/cc/work",
		"/1/work":           "/1/work",
		"c/work":            "c/work",
		"/\u00e9/work":      "/\u00e9/work",
		`C:\work`:           `C:\work`,
	} {
		if got := windowsDrive(name); got != want {
			t.Errorf("%q: %q, want %q", name, got, want)
		}
	}
}

// TestAbsoluteStartsFromTheHome places a name that is ~ or starts with ~/
// under the home, refuses it when the home is unknown, and leaves a tilde
// anywhere else to the working directory, as the host does.
func TestAbsoluteStartsFromTheHome(t *testing.T) {
	t.Parallel()
	home, cwd := filepath.Join(string(filepath.Separator)+"h", "u"), filepath.Join(string(filepath.Separator)+"w", "s")
	for _, tc := range []struct{ name, want string }{
		{"~", home},
		{"~/notes.md", filepath.Join(home, "notes.md")},
		{"\t~/../x/a.md\n", filepath.Join(filepath.Dir(home), "x", "a.md")},
		{"~someone/a.md", filepath.Join(cwd, "~someone", "a.md")},
		{"docs/~/a.md", filepath.Join(cwd, "docs", "~", "a.md")},
	} {
		if got, err := absolute(home, cwd, tc.name); err != nil || got != tc.want {
			t.Errorf("%q: %q, %v, want %q", tc.name, got, err, tc.want)
		}
	}
	for _, name := range []string{"~", "~/notes.md"} {
		if got, err := absolute("", cwd, name); !errors.Is(err, model.ErrCannotJudge) {
			t.Errorf("%q from an unknown home: %q, %v", name, got, err)
		}
	}
}

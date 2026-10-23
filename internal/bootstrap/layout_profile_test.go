package bootstrap_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestProfileLayoutFillsOnlyWhatTheRepositoryShowsNothingFor(t *testing.T) {
	t.Parallel()
	defaults := &model.Layout{Main: bootstrap.MainCmd, Decisions: "docs/decisions"}
	for _, tc := range []struct {
		name     string
		entries  []string
		defaults *model.Layout
		want     bootstrap.Layout
	}{
		{"red an empty repository", nil, defaults, bootstrap.Layout{Main: bootstrap.MainCmd, Decisions: "docs/decisions", Go: true}},
		{"near a main package at the root", []string{"main.go"}, defaults, bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/decisions", Go: true}},
		{"near a cmd directory", []string{"cmd/"}, &model.Layout{Main: bootstrap.MainRoot}, bootstrap.Layout{Main: bootstrap.MainCmd, Decisions: "docs/adr", Go: true}},
		{"near the adr directory", []string{"docs/adr/"}, defaults, bootstrap.Layout{Main: bootstrap.MainCmd, Decisions: "docs/adr", Go: true}},
		{"near the decisions directory", []string{"docs/decisions/"}, &model.Layout{Decisions: "records"}, bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/decisions", Go: true}},
		{"near no profile layout", nil, nil, bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/adr", Go: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			for _, entry := range append([]string{"go.mod"}, tc.entries...) {
				requireEntry(t, root, entry)
			}
			detected, err := bootstrap.Detect(root)
			if err != nil {
				t.Fatal(err)
			}
			if got := bootstrap.ProfileLayout(root, detected, tc.defaults); got != tc.want {
				t.Fatalf("ProfileLayout = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// requireEntry creates a directory for a name ending in a slash and a file
// for any other name, under root.
func requireEntry(t *testing.T, root, entry string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(entry))
	if strings.HasSuffix(entry, "/") {
		if err := os.MkdirAll(path, 0o750); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err := os.WriteFile(path, []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

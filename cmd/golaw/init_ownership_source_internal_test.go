package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
)

func TestInitOwnershipLocationPrecedence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			"top location wins",
			map[string]string{
				".github/CODEOWNERS": "* @top\n",
				"CODEOWNERS":         "* @root\n",
				"docs/CODEOWNERS":    "* @docs\n",
			},
			"* @top\n",
		},
		{
			"root beats docs",
			map[string]string{
				"CODEOWNERS":      "* @root\n",
				"docs/CODEOWNERS": "* @docs\n",
			},
			"* @root\n",
		},
		{"docs only", map[string]string{"docs/CODEOWNERS": "* @docs\n"}, "* @docs\n"},
		{"absent", map[string]string{}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, committed := range []bool{false, true} {
				requireOwnershipLocation(t, tc.files, tc.want, committed)
			}
		})
	}
}

// requireOwnershipLocation requires init to read the CODEOWNERS of the
// highest location, from HEAD when the files are committed, with the
// checkout differing, and from the checkout otherwise.
func requireOwnershipLocation(t *testing.T, files map[string]string, want string, committed bool) {
	t.Helper()
	repo := gittest.New(t)
	for name, content := range files {
		repo.Write(name, content)
	}
	if committed {
		repo.Commit("chore: establish ownership", files)
		repo.Write(".github/CODEOWNERS", "* @checkout\n")
	}
	ta := initBindApp(t)
	got, err := ta.bind().initOwnership(t.Context(), repo.Root)
	if err != nil || string(got) != want {
		t.Fatalf("committed=%v: ownership = %q, %v", committed, got, err)
	}
}

func TestInitOwnershipUsesCheckoutWhenDiscoveryFails(t *testing.T) {
	t.Parallel()
	root := checkout(t)
	want := "* @core\n/.github/ @platform\n"
	if err := os.WriteFile(filepath.Join(root, "CODEOWNERS"), []byte(want), 0o600); err != nil {
		t.Fatal(err)
	}
	ta := initBindApp(t)
	got, err := ta.bind().initOwnership(t.Context(), root)
	if err != nil || string(got) != want {
		t.Fatalf("ownership = %q, %v", got, err)
	}
}

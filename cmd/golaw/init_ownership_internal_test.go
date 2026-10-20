package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
)

// ownedCase is one CODEOWNERS init meets: the committed and the checkout
// owners, the diagnostic and the exit code it answers, and which of the
// policy and the protected asset it writes.
type ownedCase struct {
	name, owners, checkoutOwners, diagnostic string
	code                                     int
	policyWritten, assetWritten              bool
}

func TestInitPreservesOwnedDestinations(t *testing.T) {
	t.Parallel()
	for _, tc := range []ownedCase{
		{
			"committed protection", "* @core\n/.github/ @platform\n",
			"* @core\n", "", exitClean, true, false,
		},
		{
			"protected policy", "* @core\n/golaw.yml @platform\n",
			"", "golaw.yml", exitFailure, false, false,
		},
		{
			"unsupported negation", "* @core\n!/.github/ @platform\n",
			"", "CODEOWNERS line 2: unsupported negation", exitClean, true, true,
		},
		{
			"unsupported bracket", "* @core\n/.github/[ab].yml @platform\n",
			"", "CODEOWNERS line 2: unsupported bracket pattern", exitClean, true, true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			repo.Commit("chore: establish ownership", map[string]string{
				"go.mod":     "module example.test/service\n\ngo 1.27\n",
				"CODEOWNERS": tc.owners,
			})
			if tc.checkoutOwners != "" {
				repo.Write("CODEOWNERS", tc.checkoutOwners)
			}
			ta := initBindApp(t)
			if got := ta.run(t.Context(), []string{"init", "--apply", repo.Root}); got != tc.code {
				t.Fatalf("exit = %d: %s", got, ta.stderr.String())
			}
			requireOwnedResult(t, ta, repo.Root, tc)
		})
	}
}

// requireOwnedResult requires the diagnostic of a row, the protected asset
// written only where the row allows it, and the policy with an allowed
// destination, or no partial initialization.
func requireOwnedResult(t *testing.T, ta testApp, root string, tc ownedCase) {
	t.Helper()
	if said := ta.stdout.String() + ta.stderr.String(); tc.diagnostic != "" && !strings.Contains(said, tc.diagnostic) {
		t.Fatalf("diagnostic = %q", said)
	}
	asset := filepath.Join(root, ".github", "dependabot.yml")
	if _, err := os.Stat(asset); tc.assetWritten != (err == nil) {
		t.Fatalf("destination written = %v, want %v: %v", err == nil, tc.assetWritten, err)
	}
	_, err := os.Stat(filepath.Join(root, "golaw.yml"))
	if !tc.policyWritten {
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("partial initialization: %v", err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "lefthook.yml")); statErr != nil {
		t.Fatalf("allowed destination missing: %v", statErr)
	}
	if !strings.Contains(ta.stdout.String(), ".github/dependabot.yml") {
		t.Fatalf("destination not named: %q", ta.stdout.String())
	}
}

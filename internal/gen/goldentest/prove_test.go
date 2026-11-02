package goldentest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/goldentest"
)

func TestProveBindsEveryLawToItsFixtures(t *testing.T) {
	t.Parallel()
	goldentest.Prove(t, "testdata/proof.yml", goldentest.Laws())
	data, err := os.ReadFile("testdata/proof.yml")
	if err != nil {
		t.Fatal(err)
	}
	for _, law := range goldentest.Laws() {
		if !strings.Contains(string(data), "law: "+law.Name+",") {
			t.Errorf("testdata/proof.yml binds no fixture to %s", law.Name)
		}
	}
}

func TestVerifyRefusesAManifestThatProvesNothing(t *testing.T) {
	t.Parallel()
	tests := map[string]struct{ path, want string }{
		"red: an unknown law":         {"testdata/prove/unknown.yml", "law no-such-law is not a template law"},
		"red: a red tree that passes": {"testdata/prove/swapped.yml", "red ../laws/s107/near does not violate S107-parameters"},
		"red: a near tree that fails": {"testdata/prove/swapped.yml", "near ../laws/s107/red violates S107-parameters"},
		"red: a missing tree":         {"testdata/prove/missing.yml", "red ../laws/absent/red"},
		"red: no entry":               {"testdata/prove/empty.yml", "no entry"},
		"red: an unknown key":         {"testdata/prove/unknown-key.yml", "field extra not found"},
		"red: no manifest":            {"testdata/prove/absent.yml", "absent.yml"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			failures := goldentest.Verify(tc.path, goldentest.Laws())
			if !strings.Contains(strings.Join(failures, "\n"), tc.want) {
				t.Fatalf("Verify(%s) = %q, want a failure holding %q", tc.path, failures, tc.want)
			}
		})
	}
}

// TestVerifyReadsEscapedFixtures proves a fixture a kit check would flag
// in the kit's own tree is read through its escape.
func TestVerifyReadsEscapedFixtures(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	escaped := "package client\n\nconst Base = \"" + "\\" + "u0068ttp://api.example.com\"\n"
	write("proof.yml", "entries:\n  - { law: no-http-literal, red: red, near: near }\n")
	write("red/base.go.esc", escaped)
	write("near/base.go", "package client\n\nconst Base = \"https://api.example.com\"\n")
	if failures := goldentest.Verify(filepath.Join(dir, "proof.yml"), goldentest.Laws()); len(failures) != 0 {
		t.Fatalf("Verify = %q, want the escaped red tree to violate no-http-literal", failures)
	}
}

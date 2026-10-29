package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var pinnedPackage = flag.String("package", "", "the installed eslint-plugin-sonarjs directory, for the drift test")

// fixturePackage lays the README and package.json fixtures out as a package.
func fixturePackage(t *testing.T, manifest string) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range map[string][]byte{"README.md": readFixture(t, "README.md"), "package.json": []byte(manifest)} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const sonarjsManifest = `{ "name": "eslint-plugin-sonarjs", "version": "4.2.2" }`

func TestRunWritesThenChecks(t *testing.T) {
	t.Parallel()
	pkg := fixturePackage(t, sonarjsManifest)
	out := filepath.Join(t.TempDir(), "sonar-index.json")
	args := []string{"-package", pkg, "-rows", filepath.Join("testdata", "rows.yml"), "-out", out}
	var stdout, stderr bytes.Buffer
	if code := run(args, &stdout, &stderr); code != exitClean {
		t.Fatalf("write: exit %d: %s", code, stderr.String())
	}
	if code := run(append(args, "-check"), &stdout, &stderr); code != exitClean || !strings.Contains(stdout.String(), "up to date") {
		t.Fatalf("check after write: exit %d: %s", code, stderr.String())
	}
	if err := os.WriteFile(out, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if code := run(append(args, "-check"), &stdout, &stderr); code != exitFindings || !strings.Contains(stderr.String(), "is stale") {
		t.Errorf("check of a stale index: exit %d: %s", code, stderr.String())
	}
}

func TestRunRefuses(t *testing.T) {
	t.Parallel()
	good := fixturePackage(t, sonarjsManifest)
	other := fixturePackage(t, `{ "name": "eslint-plugin-other", "version": "1.0.0" }`)
	out := filepath.Join(t.TempDir(), "sonar-index.json")
	rows := filepath.Join("testdata", "rows.yml")
	cases := []struct {
		name  string
		args  []string
		code  int
		token string
	}{
		{"no package flag", []string{"-out", out}, exitFailure, "-package"},
		{"a missing package", []string{"-package", filepath.Join(good, "absent"), "-rows", rows, "-out", out}, exitFailure, "package.json"},
		{"another package", []string{"-package", other, "-rows", rows, "-out", out}, exitFailure, "eslint-plugin-sonarjs"},
		{"a row the README contradicts", []string{"-package", good, "-rows", filepath.Join("testdata", "rows_wrong.yml"), "-out", out}, exitFindings, "javascript:S3358 sonarjs/no-unenclosed-multiline-block"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			if code := run(c.args, &stdout, &stderr); code != c.code || !strings.Contains(stderr.String(), c.token) {
				t.Errorf("exit %d, stderr %q; want exit %d naming %q", code, stderr.String(), c.code, c.token)
			}
		})
	}
}

// The drift test of M6.4.2: the committed index equals the one the pinned
// package gives. It needs the installed package, so it skips without it.
func TestCommittedIndexMatchesThePinnedPackage(t *testing.T) {
	t.Parallel()
	if *pinnedPackage == "" {
		t.Skip("eslint-plugin-sonarjs 4.2.2 not given: run golaw setup --surface, then go test ./tools/sonarmap -args -package <node_modules>/eslint-plugin-sonarjs")
	}
	args := []string{
		"-package", *pinnedPackage,
		"-rows", filepath.Join("..", "..", "internal", "lint", "rulemap", "web.yml"),
		"-out", filepath.Join("..", "..", "internal", "lint", "config", "web", "sonar-index.json"),
		"-check",
	}
	var stdout, stderr bytes.Buffer
	if code := run(args, &stdout, &stderr); code != exitClean {
		t.Errorf("exit %d: %s", code, stderr.String())
	}
}

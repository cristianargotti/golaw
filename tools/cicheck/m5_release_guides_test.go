package cicheck_test

import (
	"strings"
	"testing"
)

const m5ReleasingPath = "docs/releasing.md"

func TestM5ReleaseVerificationGuides(t *testing.T) {
	t.Parallel()
	setup := []string{
		"## Verify a release",
		"gh attestation download golaw_<version>_<os>-<arch> --repo cristianargotti/golaw\n",
		"./golaw_<version>_<os>-<arch> doctor --release .\n",
		"sha256:<hex>.jsonl, or sha256-<hex>.jsonl on Windows",
		"make release-verify BIN=<downloaded binary> DIST=<dir>",
		"any other result is red, names the file and the cause, and exits 1.",
		"--trust-root <dir>",
		"doctor --release runs inside the binary it judges",
		"source build, executable not verifiable",
		"A source build or a go install build is never verified: doctor --release answers unknown [source-build] and exits 0 whatever directory it is given, and so does make release-verify with such a binary.",
	}
	readme := []string{
		"verify it with golaw doctor --release <dir> as docs/setup.md (Verify a release) explains",
		"docs/profiles.md",
	}
	releasing := []string{
		"## Replay the gate",
		"gh api --method GET repos/cristianargotti/golaw/actions/workflows/ci.yml/runs -f head_sha=<commit> -f branch=main -f event=push -f per_page=100 > runs.json\n",
		"make release-gate-dry-run SHA=<commit> RUNS=runs.json\n",
		"Neither release-gate-dry-run nor release-verify is part of make ci.",
		"make release-verify BIN=<downloaded binary> DIST=<dir>",
	}
	stale := []string{"automatic release verification in doctor arrives in M5"}
	guide, page, front := string(kitBytes(t, m5SetupPath)), string(kitBytes(t, m5ReleasingPath)), string(kitBytes(t, "README.md"))
	rows := []struct {
		name, path, text string
		required         []string
		want             bool
	}{
		{"setup", m5SetupPath, guide, setup, true},
		{"releasing", m5ReleasingPath, page, releasing, true},
		{"red-stale-promise", m5SetupPath, guide + "\nVerify it yourself; " + stale[0] + ".\n", setup, false},
		{"red-missing-trust-root", m5SetupPath, strings.ReplaceAll(guide, setup[6], ""), setup, false},
		{"red-missing-source-build-exception", m5SetupPath, strings.ReplaceAll(guide, setup[9], ""), setup, false},
		{"readme", "README.md", front, readme, true},
		{"red-readme-stale-promise", "README.md", front + "\nVerify it yourself; " + stale[0] + ".\n", readme, false},
		{"red-missing-dry-run", m5ReleasingPath, strings.Replace(page, releasing[2], "", 1), releasing, false},
		{"near-cosign-kept", m5SetupPath, guide + "\nThe issuer is https://token.actions.githubusercontent.com.\n", setup, true},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if got := m4TextOK(row.text, row.required, stale); got != row.want {
				t.Fatalf("%s = %v, want %v", row.path, got, row.want)
			}
		})
	}
	requireText(t, "Makefile", "release-gate-dry-run:\n", "release-verify:\n")
}

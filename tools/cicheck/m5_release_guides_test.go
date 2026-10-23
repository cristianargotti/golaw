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
	}
	releasing := []string{
		"## Replay the gate",
		"gh api --method GET repos/cristianargotti/golaw/actions/workflows/ci.yml/runs -f head_sha=<commit> -f branch=main -f event=push -f per_page=100 > runs.json\n",
		"make release-gate-dry-run SHA=<commit> RUNS=runs.json\n",
		"Neither release-gate-dry-run nor release-verify is part of make ci.",
		"make release-verify BIN=<downloaded binary> DIST=<dir>",
	}
	stale := []string{"automatic release verification in doctor arrives in M5"}
	guide, page := string(kitBytes(t, m5SetupPath)), string(kitBytes(t, m5ReleasingPath))
	rows := []struct {
		name, path, text string
		required         []string
		want             bool
	}{
		{"setup", m5SetupPath, guide, setup, true},
		{"releasing", m5ReleasingPath, page, releasing, true},
		{"red-stale-promise", m5SetupPath, guide + "\nVerify it yourself; " + stale[0] + ".\n", setup, false},
		{"red-missing-trust-root", m5SetupPath, strings.ReplaceAll(guide, setup[6], ""), setup, false},
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

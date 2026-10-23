package cicheck_test

import (
	"slices"
	"strings"
	"testing"
)

// gateDecision is the line of the gate script that decides, and gateLines
// the lines it must keep in order around it (D12).
const gateDecision = `go run ./tools/releasegate -sha "$tag_sha" -runs "$RUNNER_TEMP/ci-runs.json"`

func gateLines() []string {
	return []string{
		"set -euo pipefail",
		`git merge-base --is-ancestor "$tag_sha" refs/remotes/origin/main`,
		`-f head_sha="$tag_sha" -f branch=main -f event=push \`,
		gateDecision,
		`printf 'sha=%s\n' "$tag_sha" >> "$GITHUB_OUTPUT"`,
	}
}

// inspectReleaseGate requires the script to keep its checks in order, to
// decide with the gate tool before it writes the eligible commit, and to
// carry no jq and no way past a failing line.
func inspectReleaseGate(script string) string {
	var lines []string
	for line := range strings.SplitSeq(script, "\n") {
		lines = append(lines, strings.TrimSpace(line))
	}
	from := 0
	for _, want := range gateLines() {
		i := slices.Index(lines[from:], want)
		if i < 0 {
			return "The release gate script lost or moved the line " + want
		}
		from += i + 1
	}
	if strings.Contains(script, "jq ") || strings.Contains(script, "||") {
		return "The release gate script decides with the gate tool alone and never ignores a failure."
	}
	return ""
}

func TestReleaseGateDecidesWithTheTool(t *testing.T) {
	t.Parallel()
	script := string(kitBytes(t, "tools/cicheck/release-gate.sh"))
	if reason := inspectReleaseGate(script); reason != "" {
		t.Fatal(reason)
	}
	for _, mutation := range [][2]string{
		{"go run ./tools/releasegate", "true"},
		{gateDecision, gateDecision + " || true"},
		{gateDecision + "\n", "jq -e . \"$RUNNER_TEMP/ci-runs.json\"\n"},
		{`"$tag_sha" -runs`, `"$GITHUB_SHA" -runs`},
		{"set -euo pipefail\n", ""},
		{"git merge-base --is-ancestor", "true"},
		{"-f event=push", "-f event=pull_request"},
		{gateDecision + "\nprintf", "printf"},
	} {
		changed := strings.Replace(script, mutation[0], mutation[1], 1)
		if changed == script {
			t.Fatalf("gate mutation did not change its target: %q", mutation[0])
		}
		if inspectReleaseGate(changed) == "" {
			t.Errorf("unguarded gate accepted: %q", mutation[1])
		}
	}
}

// gateOrder answers the indexes of the setup-go step, the step that
// restores the automatic toolchain and the eligible step of the gate job.
func gateOrder(data []byte) (int, int, int) {
	root, err := document(data)
	if err != nil {
		return -1, -1, -1
	}
	setup, restore, eligible := -1, -1, -1
	for i, step := range children(at(root, "jobs", "gate", "steps")) {
		switch {
		case strings.HasPrefix(value(at(step, "uses")), "actions/setup-go@"):
			setup = i
		case value(at(step, "name")) == "Restore automatic toolchain selection":
			restore = i
		case value(at(step, "id")) == "eligible":
			eligible = i
		}
	}
	return setup, restore, eligible
}

// inspectGateOrder requires the gate job to set up the pinned Go and
// restore the automatic toolchain right before the eligible step, because
// the gate decides with Go: a runner's own Go never decides.
func inspectGateOrder(data []byte) string {
	if setup, restore, eligible := gateOrder(data); setup < 0 || restore != setup+1 || eligible != restore+1 {
		return "The gate job sets up Go and restores the automatic toolchain right before the eligible step."
	}
	return ""
}

func TestTheGateJobSetsUpGoBeforeItDecides(t *testing.T) {
	t.Parallel()
	data := string(kitBytes(t, ".github/workflows/release.yml"))
	if reason := inspectGateOrder([]byte(data)); reason != "" {
		t.Fatal(reason)
	}
	eligible := "      - id: eligible\n        name: Require successful CI for the tag commit\n        shell: bash\n" +
		"        env:\n          GH_TOKEN: ${{ github.token }}\n        run: bash tools/cicheck/release-gate.sh\n"
	restore := "      - name: Restore automatic toolchain selection\n"
	first := strings.Replace(strings.Replace(data, eligible, "", 1), "      - uses: actions/setup-go@", eligible+"      - uses: actions/setup-go@", 1)
	for name, changed := range map[string]string{
		"the gate decides first": first,
		"no restored toolchain":  strings.Replace(data, restore, "      - name: Keep the runner toolchain\n", 1),
	} {
		if changed == data {
			t.Fatalf("%s: the mutation did not change the file", name)
		}
		if inspectGateOrder([]byte(changed)) == "" {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestMakefileReplaysTheGateAndVerifiesWithTheDownloadedBinary(t *testing.T) {
	t.Parallel()
	requireText(t, "Makefile",
		"release-gate-dry-run:\n\tgo run ./tools/releasegate -sha $(SHA) -runs $(RUNS)\n",
		"release-verify:\n\t$(BIN) doctor --release $(DIST)\n",
		" vuln release-gate-dry-run release-verify\n",
	)
	for line := range strings.SplitSeq(string(kitBytes(t, "Makefile")), "\n") {
		if strings.HasPrefix(line, "ci:") && strings.Contains(line, "release-") {
			t.Fatalf("a release target is in ci: %s", line)
		}
	}
}

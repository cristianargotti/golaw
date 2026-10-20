package cicheck_test

import (
	"regexp"
	"strings"
	"testing"
)

func TestM2EvidenceBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		text string
		want bool
	}{
		{"docs/agent-contracts.md", "M2 rung acceptance runs on Linux, macOS and Windows.", true},
		{"docs/agent-contracts.md", "Windows rung CI evidence does not establish live agent enforcement.", true},
		{"docs/agent-contracts.md", "Required rung rows must pass; SKIP never counts as PASS.", true},
		{"docs/agent-contracts.md", "The installed-rung rows invoke the installed git hook routing.", true},
		{"docs/agent-contracts.md", "A simulated tool result proves the harness path, not real tool latency.", true},
		{"docs/agent-contracts.md", "With Git's shell as the only shell on the constrained PATH, the Windows row proves that the rungs never start make, awk or tar.", true},
		{"docs/agent-contracts.md", "Each name resolves to its recorder in the same shell before the rungs run, and all recorder logs remain empty afterward.", true},
		{"docs/agent-contracts.md", "M2 proves every agent on Windows.", false},
		{"docs/acceptance/m2.md", "M2_MEASUREMENTS_COMPLETE", true},
		{"docs/acceptance/m2.md", "GOLAW_M2_MEASURE=1", true},
		{"docs/acceptance/m2.md", "GOCACHE", true},
		{"docs/acceptance/m2.md", "GOMODCACHE", true},
		{"docs/acceptance/m2.md", "GOLANGCI_LINT_CACHE", true},
		{"docs/acceptance/m2.md", "Cache definitions use names only and never publish paths from the measuring machine.", true},
		{"docs/acceptance/m2.md", "An unavailable measurement remains unmeasured and is never published as zero.", true},
		{"docs/acceptance/m2.md", "The native m2 CI job publishes measurements; release acceptance does not rerun them.", true},
		{"docs/acceptance/m2.md", "Each warm median is compared with the 10-second budget of its rung; a rung over it fails the m2 job.", true},
		{"docs/acceptance/m2.md", "The required m2 job times the push rung against a fixture database; live-database timings come from the scheduled or manual m2-live job.", true},
		{"docs/rules.md", "commit.author-noreply is deferred: M2 never infers that a repository is public.", true},
		{"docs/rules.md", "M2 does not add a remote commit-range message gate.", true},
		{"docs/rules.md", "Migration, API and generation rung tools arrive with the M3 templates.", true},
	}
	for _, tt := range tests {
		text := string(kitBytes(t, tt.path))
		if got := strings.Contains(text, tt.text); got != tt.want {
			t.Errorf("%s: fragment %q present=%v, want=%v",
				tt.path, tt.text, got, tt.want)
		}
	}
}

func TestM2ChangelogNamesBehaviorAndReason(t *testing.T) {
	t.Parallel()
	lines := []string{
		"scan --staged compares index findings with the base as a multiset and evaluates the proposed policy separately, so unstaged edits cannot hide a staged violation.",
		"scan --enrolled accepts externally provisioned digests, because a clean CI runner must not trust enrollment supplied by its checkout.",
		"the optional pre-commit, commit-msg and pre-push rungs run through the binary, because local checks must not depend on make, awk or tar.",
		"the commits family, rule rules/4.12@1, adds commit.conventional, commit.no-trailers, commit.no-dashes, commit.body-width and commit.shape, because git message files need the same commit laws as supported agent commands.",
		"init writes core templates without arming hooks, and setup installs pinned tools and arms the chosen rungs, because every local check remains the developer's choice.",
		"init --update previews managed-file changes and the exact version pin, preserves local edits and applies reviewed updates, because differing binary versions must not govern one repository.",
		"golaw coverage carries the total, package and file floors over the build universe, because the public command replaces the kit-only coverage tool.",
		"M2 acceptance records staged-state adversaries, setup recovery, optional switches, version mismatches and rung measurements, because behavior and latency need reproducible evidence.",
		"doctor reports managed drift, a managed file edited or gone from the checkout, because a file init manages must stay visible after a local change.",
	}
	text := string(kitBytes(t, "CHANGELOG.md"))
	for _, line := range lines {
		pattern := regexp.MustCompile(`(?m)^- \d{4}-\d{2}-\d{2}: ` +
			regexp.QuoteMeta(line) + `$`)
		if !pattern.MatchString(text) {
			t.Errorf("changelog lacks a dated line for %q", line)
		}
		for _, tt := range []struct {
			name string
			text string
			want bool
		}{
			{"near date", "- 2026-10-20: " + line, true},
			{"another date", "- 2027-01-03: " + line, true},
			{"red absent date", "- " + line, false},
			{"corrupted date", "- 2026-1-3: " + line, false},
			{"changed behavior", "- 2026-10-20: removed", false},
		} {
			if got := pattern.MatchString(tt.text); got != tt.want {
				t.Errorf("%s: match=%v, want=%v", tt.name, got, tt.want)
			}
		}
	}
}

func TestM2MeasurementPageKeepsMachinePathsPrivate(t *testing.T) {
	t.Parallel()
	if homePath.Match(kitBytes(t, "docs/acceptance/m2.md")) {
		t.Fatal("measurement page carries an absolute home path")
	}
}

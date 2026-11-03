package cicheck_test

import (
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestSetupExplainsTheRepairAndTheEvidence pins what the setup page says
// about a reviewed policy change, the doctor lines and a downloaded asset.
func TestSetupExplainsTheRepairAndTheEvidence(t *testing.T) {
	t.Parallel()
	requireText(t, "docs/setup.md",
		"init --apply never renders over a golaw.yml the checkout already holds",
		"until agent install enrolls the policy in force",
		"The record follows a tightening only",
		"reports a newer host as unknown until it is validated",
		"The final line gives the time and the OS of the last hook process",
		"Install the verified asset on your PATH under the name golaw",
	)
}

// TestReadmePinsTheInstall holds the front page to section 4.2: the source
// build is pinned to the release the setup page installs and never
// latest, the page links the day one sequence, and a downloaded release
// asset is installed under the name the plugin runs from PATH.
func TestReadmePinsTheInstall(t *testing.T) {
	t.Parallel()
	pinned := regexp.MustCompile(`go install github\.com/cristianargotti/golaw/cmd/golaw@(v\d+\.\d+\.\d+)\n`)
	readme := pinned.FindStringSubmatch(string(kitBytes(t, "README.md")))
	setup := pinned.FindStringSubmatch(string(kitBytes(t, "docs/setup.md")))
	if readme == nil || setup == nil || readme[1] != setup[1] {
		t.Fatalf("README pins %v, docs/setup.md pins %v", readme, setup)
	}
	if strings.Contains(string(kitBytes(t, "README.md")), "@latest") {
		t.Fatal("README uses an unpinned source build")
	}
	requireText(t, "README.md",
		"docs/setup.md",
		"golaw init --apply",
		"golaw agent install --agent "+agentID(t),
		"under the name golaw",
		"checksums.txt",
	)
}

// TestSkillReadsTheRulesFromTheBinary holds the skill to the rules every
// adopting repository can read: init writes no docs/rules.md there, and an
// agent sent to a page fetched it from the network in the bench, so the
// skill names the command that prints the pages the binary embeds.
func TestSkillReadsTheRulesFromTheBinary(t *testing.T) {
	t.Parallel()
	skill := string(kitBytes(t, "agents/SKILL.md"))
	for _, want := range []string{"`golaw rules <id>`", "`golaw rules --checklist <name>`", "never fetch them from GitHub"} {
		if !strings.Contains(skill, want) {
			t.Errorf("the skill does not name %s", want)
		}
	}
	for _, stale := range []string{"Read docs/rules.md", "blob/v", "Read docs/checklists.md"} {
		if strings.Contains(skill, stale) {
			t.Errorf("the skill points at a page adopting repositories do not hold: %s", stale)
		}
	}
}

// TestPolicyPinsTheReleaseTheSetupInstalls holds the kit's own golaw.yml
// to the tag the setup page installs: the release lane scans the tagged
// commit with the candidate it built, and that binary refuses a policy
// that pins another kit version, so a stale pin stops the release.
func TestPolicyPinsTheReleaseTheSetupInstalls(t *testing.T) {
	t.Parallel()
	root, err := document(kitBytes(t, "golaw.yml"))
	if err != nil {
		t.Fatal(err)
	}
	pin := at(root, "kit", "version")
	setup := regexp.MustCompile(`golaw@v(\d+\.\d+\.\d+)\n`).FindStringSubmatch(string(kitBytes(t, "docs/setup.md")))
	if pin == nil || setup == nil || pin.Value != setup[1] {
		t.Fatalf("golaw.yml pins %q, docs/setup.md installs %v", nodeValue(pin), setup)
	}
	requireText(t, "docs/releasing.md", "kit.version of golaw.yml")
}

// nodeValue is the scalar of a node the test may not find.
func nodeValue(node *yaml.Node) string {
	if node == nil {
		return ""
	}
	return node.Value
}

// TestContributingArmsTheRungsWithSetup holds the contributor page to the
// way M2 arms a rung: golaw hook passes silently until golaw setup records
// its receipt, so a raw lefthook install arms nothing.
func TestContributingArmsTheRungsWithSetup(t *testing.T) {
	t.Parallel()
	requireText(t, "CONTRIBUTING.md", "`golaw setup`", "`golaw setup --uninstall`")
	if strings.Contains(string(kitBytes(t, "CONTRIBUTING.md")), "lefthook install") {
		t.Fatal("CONTRIBUTING.md arms the rungs with lefthook install, which arms nothing")
	}
}

package cicheck_test

import (
	"regexp"
	"strings"
	"testing"
)

func TestM1SetupSequence(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, "docs/setup.md"))
	agent := agentID(t)
	commands := []string{
		"go install github.com/cristianargotti/golaw/cmd/golaw@v0.1.0",
		"golaw init\n",
		"golaw init --apply",
		"git add golaw.yml",
		"git commit",
		"golaw agent install --agent " + agent,
		"golaw doctor",
		"golaw scan .",
	}
	position := 0
	for _, command := range commands {
		offset := strings.Index(text[position:], command)
		if offset < 0 {
			t.Fatalf("setup command missing or out of order: %s", command)
		}
		position += offset + len(command)
	}
	if strings.Contains(text, "@latest") {
		t.Fatal("setup uses an unpinned source build")
	}
	requireText(t, "docs/setup.md",
		"no templates",
		"Commit the reviewed golaw.yml before agent install",
		"the policy in force is the golaw.yml at HEAD",
		"never arms git hooks",
		"source build, executable not verifiable",
		"automatic release verification in doctor arrives in M5",
		"only hard gate",
		"golaw agent uninstall --agent "+agent,
		"M1 does not provide golaw setup or scan --staged",
	)
}

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

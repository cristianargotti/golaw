package cicheck_test

import (
	"strings"
	"testing"
)

func TestM2SetupSequence(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, "docs/setup.md"))
	agent := agentID(t)
	commands := []string{
		"go install github.com/cristianargotti/golaw/cmd/golaw@v0.3.0\n",
		"golaw init\n",
		"golaw init --apply\n",
		"git add golaw.yml",
		"git commit -m \"feat: adopt the golaw policy\"\n",
		"golaw setup\n",
		"golaw agent install --agent " + agent + "\n",
		"golaw doctor\n",
		"golaw scan --staged\n",
	}
	position := 0
	for _, command := range commands {
		offset := strings.Index(text[position:], command)
		if offset < 0 {
			t.Fatalf("setup command missing or out of order: %s", command)
		}
		position += offset + len(command)
	}
	requireText(t, "docs/setup.md",
		"Commit the reviewed golaw.yml before agent install",
		"the policy in force is the golaw.yml at HEAD",
		"never arms git hooks",
		"source build, executable not verifiable",
		"only hard gate",
		"golaw agent uninstall --agent "+agent,
		"The adoption commit stages every file listed in the reviewed init diff.",
		"The committed lefthook.yml lets teammates opt into the same rungs with golaw setup.",
		"The adoption commit and every init --update commit use the feat type, because commit.shape holds a commit that is not feat to 40 new files and 400 new lines.",
	)
}

func TestM2SetupAndSecurityBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		text string
		want bool
	}{
		{"docs/setup.md", "golaw setup is optional.", true},
		{"docs/setup.md", "init never arms git hooks.", true},
		{"docs/setup.md", "A repeated init --apply produces no diff.", true},
		{"docs/setup.md", "An interrupted setup leaves no receipt; run golaw setup again to finish.", true},
		{"docs/setup.md", "test_staged is off by default.", true},
		{"docs/setup.md", "An exact kit.version mismatch is refused in either direction.", true},
		{"docs/setup.md", "Local edits are kept and reported.", true},
		{"docs/setup.md", "doctor reports managed drift: a managed file whose bytes differ from the hash init recorded, which golaw init --update shows and keeps, and a managed file gone from the checkout, which git restore --source=HEAD --staged --worktree -- <path> brings back.", true},
		{"docs/setup.md", "A git client started without GOBIN on its PATH, as a graphical client may be, cannot run golaw or lefthook from the armed hooks: the commit fails or the rung is skipped, a stated limit; doctor reports the golaw path setup recorded, and starting the client from a shell with GOBIN on PATH restores the rungs.", true},
		{"docs/setup.md", "M1 does not provide golaw setup or scan --staged", false},
		{"docs/setup.md", "@latest", false},
		{"docs/security.md", "Each ladder switch can be false without a finding, before version checks or tool resolution.", true},
		{"docs/security.md", "The pre-push rung runs govulncheck only.", true},
		{"docs/security.md", "A scanner failure or unreachable database is reported as Unmeasured and does not block this rung.", true},
		{"docs/security.md", "User commands lose GIT_DIR, GIT_WORK_TREE, GIT_INDEX_FILE, GIT_COMMON_DIR and GIT_PREFIX; the kit's plumbing keeps them.", true},
		{"docs/security.md", "A false denial or a false finding is a defect.", true},
		{"docs/security.md", "Unsupported exotic constructs are stated limits, never a reason to widen a reader.", true},
		{"agents/SKILL.md", "Run golaw scan --staged before finishing staged work.", true},
		{"agents/SKILL.md", "When golaw scan --staged exists in your version", false},
	}
	for _, tt := range tests {
		text := string(kitBytes(t, tt.path))
		if got := strings.Contains(text, tt.text); got != tt.want {
			t.Errorf("%s: fragment %q present=%v, want=%v",
				tt.path, tt.text, got, tt.want)
		}
	}
}

func TestM2RemoteEnrollmentSequence(t *testing.T) {
	t.Parallel()
	requireText(t, "docs/setup.md",
		"A runner without a local enrollment record uses scan --enrolled <digests>, with expected policy and optional profile digests provisioned outside the checkout by a repository variable or pipeline setting.",
		"The value is a 64-character lowercase hexadecimal policy digest, optionally followed by a comma and a 64-character lowercase hexadecimal profile digest.",
		"The supplied digests replace the local enrollment lookup for this scan.",
		"A missing policy at HEAD or a digest mismatch is refused.",
		"Do not derive the expected digests from the candidate checkout.",
	)
}

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
)

// longGo is a Go file over the file limit of goast.file-length, a check
// the go capability conditions.
func longGo() string {
	return "package m\n\n" + strings.Repeat("// line\n", 300)
}

// capabilityCases are the capability lines a repository commits, and
// whether goast.file-length judges it under each.
func capabilityCases() map[string]struct {
	line   string
	judged bool
} {
	return map[string]struct {
		line   string
		judged bool
	}{
		"go":            {"capabilities: [go]\n", true},
		"go and image":  {"capabilities: [go, image]\n", true},
		"image alone":   {"capabilities: [image]\n", false},
		"no capability": {"capabilities: []\n", false},
	}
}

func TestScanAndTheStagedScanRunTheGoChecksOnlyUnderGo(t *testing.T) {
	t.Parallel()
	for name, tc := range capabilityCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := capabilityRepository(t, tc.line)
			stageFiles(t, root, map[string]string{"long.go": longGo()})
			staged := stagedApp(t)
			code := staged.run(t.Context(), []string{"scan", "--staged", root})
			if judged := strings.Contains(staged.stdout.String(), "goast.file-length"); judged != tc.judged || (code == exitFindings) != tc.judged {
				t.Fatalf("staged scan: exit %d, stdout %q, stderr %q", code, staged.stdout.String(), staged.stderr.String())
			}
			gitRun(t, root, "commit", "-q", "-m", "feat: add the long file")
			scan := boundApp(t)
			code = scan.run(t.Context(), []string{"scan", root})
			if judged := strings.Contains(scan.stdout.String(), "goast.file-length"); judged != tc.judged || (code == exitFindings) != tc.judged {
				t.Fatalf("scan: exit %d, stdout %q, stderr %q", code, scan.stdout.String(), scan.stderr.String())
			}
		})
	}
}

func TestTheAgentHookRunsTheGoChecksOnlyUnderGo(t *testing.T) {
	t.Parallel()
	for name, tc := range capabilityCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b, _ := testBinding(t)
			root := capabilityRepository(t, tc.line)
			data := []byte(strings.Replace(string(renderedPolicy(t)), "capabilities: [go]\n", tc.line, 1))
			p, err := b.compile(data, now())
			if err != nil {
				t.Fatal(err)
			}
			if err = b.enroll(root, data, p, now()); err != nil {
				t.Fatal(err)
			}
			deps, err := b.hookDependencies(agentUnderTest(), now())
			if err != nil {
				t.Fatal(err)
			}
			event := eventAt(t, "Write", root, map[string]any{"file_path": filepath.Join(root, "long.go"), "content": longGo()})
			agent := agentUnderTest()
			reply := hook.Run(t.Context(), agent.ID, strings.NewReader(event), agent.Adapter, deps)
			denied := strings.Contains(string(reply.Stdout), `"permissionDecision":"deny"`)
			if reply.ExitCode != 0 || denied != tc.judged || (denied && !strings.Contains(string(reply.Stdout), "goast.file-length")) {
				t.Fatalf("exit %d, stdout %q, stderr %q", reply.ExitCode, reply.Stdout, reply.Stderr)
			}
		})
	}
}

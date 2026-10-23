package doctor

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestAgentsChainLinesJudgeTheRootFileCodexReadsFirst(t *testing.T) {
	t.Parallel()
	root := filepath.Join("repo", "AGENTS.md")
	for _, tc := range []struct {
		name        string
		chain       chainFact
		state, code string
		detail      []string
	}{
		{"red over the default budget", chainFact{Path: root, Size: 40000, Budget: chainBudget}, stateRed, codeChainOver, []string{root, "40000", "32768", "golaw block"}},
		{"red over a configured budget", chainFact{Path: root, Size: 20000, Budget: 16384}, stateRed, codeChainOver, []string{root, "20000", "16384"}},
		{"near at the budget", chainFact{Path: root, Size: chainBudget, Budget: chainBudget}, stateOK, codeChainWithin, []string{root, "32768", "below the root"}},
		{"near no file at the root", chainFact{Budget: chainBudget}, stateOK, codeChainWithin, []string{"no AGENTS.md"}},
		{"unread", chainFact{Error: "permission denied", Budget: chainBudget}, stateUnknown, codeChainUnread, []string{"permission denied"}},
	} {
		facts := installedFacts(agentCodex, "user")
		facts.Chain = tc.chain
		lines := agentsChainLines(facts)
		if len(lines) != 1 || lines[0].ID != idAgentsChain || lines[0].Key != "codex/user" || lines[0].State != tc.state || lines[0].Code != tc.code {
			t.Fatalf("%s: %+v", tc.name, lines)
		}
		for _, want := range tc.detail {
			if !strings.Contains(lines[0].Detail, want) {
				t.Fatalf("%s: detail %q lacks %q", tc.name, lines[0].Detail, want)
			}
		}
	}
	over := chainFact{Path: root, Size: 40000, Budget: chainBudget}
	cursor := installedFacts(agentCursor, "user")
	cursor.Chain = over
	absent := installedFacts(agentCodex, "user")
	absent.Receipt.State, absent.Chain = receiptAbsent, over
	if agentsChainLines(cursor) != nil || agentsChainLines(absent) != nil {
		t.Fatal("a chain line for an agent without a Codex receipt")
	}
}

func TestCollectChainReadsTheFileAndTheBudgetCodexReads(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		files         map[string]string
		path          string
		size, budget  int64
		error         bool
		overrideIsDir bool
	}{
		{"no file", nil, "", 0, chainBudget, false, false},
		{"the root AGENTS.md", map[string]string{"repo/AGENTS.md": "# Notes\n"}, "repo/AGENTS.md", 8, chainBudget, false, false},
		{"the override first", map[string]string{"repo/AGENTS.md": "# Notes\n", "repo/AGENTS.override.md": "# Mine\n"}, "repo/AGENTS.override.md", 7, chainBudget, false, false},
		{"an override directory skipped", map[string]string{"repo/AGENTS.md": "# Notes\n"}, "repo/AGENTS.md", 8, chainBudget, false, true},
		{"the user budget", map[string]string{"home/config.toml": "project_doc_max_bytes = 65536\n"}, "", 0, 65536, false, false},
		{"the largest budget set", map[string]string{"home/config.toml": "project_doc_max_bytes = 65536\n", "repo/.codex/config.toml": "project_doc_max_bytes = 16384\n"}, "", 0, 65536, false, false},
		{"a smaller budget set", map[string]string{"repo/.codex/config.toml": "project_doc_max_bytes = 16384\n"}, "", 0, 16384, false, false},
		{"a malformed configuration", map[string]string{"home/config.toml": "project_doc_max_bytes = ["}, "", 0, chainBudget, true, false},
	} {
		dir := t.TempDir()
		for name, content := range tc.files {
			writeFile(t, filepath.Join(dir, name), content)
		}
		if tc.overrideIsDir {
			if err := os.MkdirAll(filepath.Join(dir, "repo", "AGENTS.override.md"), 0o750); err != nil {
				t.Fatal(err)
			}
		}
		got := collectChain(Input{Root: filepath.Join(dir, "repo"), ConfigDir: filepath.Join(dir, "home")})
		want := ""
		if tc.path != "" {
			want = filepath.Join(dir, tc.path)
		}
		if got.Path != want || got.Size != tc.size || got.Budget != tc.budget || (got.Error != "") != tc.error {
			t.Fatalf("%s: %+v", tc.name, got)
		}
	}
}

func TestCodexReportMeasuresTheRootAgentsFile(t *testing.T) {
	t.Parallel()
	in := installCodex(t)
	in.input.Root, in.input.ConfigDir = t.TempDir(), in.dir
	writeFile(t, filepath.Join(in.input.Root, "AGENTS.md"), strings.Repeat("x", chainBudget+1))
	got := lineWith(Codex(t.Context(), in.input, &enrollmentStore{}).Lines, idAgentsChain)
	if got.State != stateRed || got.Code != codeChainOver || !strings.Contains(got.Detail, strconv.Itoa(chainBudget+1)) {
		t.Fatalf("chain = %+v", got)
	}
}

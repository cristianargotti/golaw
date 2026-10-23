package cicheck_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

const (
	m3RulesPath = "docs/rules.md"
	m3SkillPath = "agents/SKILL.md"
)

func TestM3RetainedProseMapping(t *testing.T) {
	t.Parallel()
	rows := []struct {
		source string
		path   string
		text   string
	}{
		{"4.1: one use case per file", m3SkillPath, "one use case per file"},
		{"4.1: pull request size and topic", m3SkillPath, "400 changed lines and one topic"},
		{"4.2: truthful why comments", m3SkillPath, "Explain why the current code needs the decision."},
		{"4.3: HTTP error identity", m3SkillPath, "errors.Is against sentinels"},
		{"4.4: third-party boundary dependencies", m3RulesPath, "A third-party dependency in domain or application needs a decision record and an explicit allowed boundary."},
		{"4.4: context shapes", m3RulesPath, "Context shapes use plain application structs, repository row mapping, generated handler signatures and validated configuration."},
		{"4.4: metric ownership", m3SkillPath, "A metric belongs to the package that emits it."},
		{"4.4: transactions and retries", m3ChecklistsPath, "Keep network calls outside transactions."},
		{"4.6: wall-clock budgets", m3ChecklistsPath, "Use benchmarks for wall-clock budgets."},
		{"4.6: mutant equivalence", m3ChecklistsPath, "An equivalence claim names the applied input and identical observed result."},
		{"4.6: growing-table query plans", m3ChecklistsPath, "generate_series, ANALYZE and EXPLAIN"},
		{"4.6: falsifiable checks and harness results", m3ChecklistsPath, "Read both the exit code and stderr."},
		{"4.6: amd64 delivery evidence", m3ChecklistsPath, "GOARCH=amd64"},
		{"4.6: workstation concurrency", m3ChecklistsPath, "Run only one heavy process at a time on the workstation."},
		{"4.9: price constraints", m3ChecklistsPath, "Price constraints use CHECK"},
		{"4.9: fingerprint versions", m3ChecklistsPath, "Store a version beside every persisted fingerprint."},
		{"4.9: finite retention", m3ChecklistsPath, "Every finite retention policy has a purge query."},
		{"4.9: pool and call budgets", m3ChecklistsPath, "statement_timeout, lock_timeout and idle_in_transaction_session_timeout"},
		{"4.9: concurrent idempotency", m3ChecklistsPath, "Test simultaneous duplicate delivery and the partial unique index."},
		{"4.10: contract identity in liveness", m3ChecklistsPath, "Expose the contract hash in liveness."},
		{"4.11: process and log identity", m3ChecklistsPath, "Use one version per process and log service, env, version and trace identifiers."},
		{"4.11: histogram ceilings", m3ChecklistsPath, "The highest finite histogram bucket covers the configured ceiling."},
		{"4.11: end-to-end deadlines", m3ChecklistsPath, "Budget deadlines from the request boundary to the database."},
		{"4.12: destination conventions", m3ChecklistsPath, "Read the destination repository's branch, title and body conventions."},
		{"4.12: squash trailer risk", m3ChecklistsPath, "Check the author address and final squash message for an added trailer."},
		{"4.15: strongest documented practice", m3SkillPath, "Follow the strongest documented practice."},
		{"4.15: reuse ladder", m3SkillPath, "existing platform capability"},
		{"4.15: managed-object justification", m3ChecklistsPath, "Each managed object names its property, count, policy family, alternatives and reversal condition."},
		{"4.15: measurement falsifiability", m3ChecklistsPath, "A check needs a red case, a near miss and an explicit input universe."},
		{"4.15: reproducible claims", m3SkillPath, "Record the command and result for each verification claim."},
		{"4.15: agreement between tools", m3ChecklistsPath, "Two tools must not disagree silently."},
		{"4.15: recurring findings and lessons", m3ChecklistsPath, "Record a lesson the same day and date its change with a reason."},
		{"4.15: verification and review", m3ChecklistsPath, "Use tools for verification and one reviewer for judgment."},
		{"4.15: builder and reviewer checklists", m3RulesPath, "The builder and reviewer use docs/checklists.md."},
	}
	text := string(kitBytes(t, m3RulesPath))
	for _, row := range rows {
		entry := "| " + row.source + " | " + row.path + " |"
		if strings.Count(text, entry) != 1 {
			t.Errorf("mapping must occur once: %s", row.source)
		}
		requireText(t, row.path, row.text)
	}
}

func TestM3SkillPointsToDeliveredGuides(t *testing.T) {
	t.Parallel()
	source := kitBytes(t, m3SkillPath)
	copyPath := "agents/" + agentID(t) + "/skills/golaw/SKILL.md"
	if !bytes.Equal(source, kitBytes(t, copyPath)) {
		t.Fatal("the embedded skill differs from its canonical source")
	}
	if reason := inspectSkill(m3SkillPath, source); reason != "" {
		t.Fatal(reason)
	}
	requireText(t, m3SkillPath,
		"Read docs/checklists.md of the kit at the same tag.",
		"Policy-file checks run in scan and staged scan, not in agent events.",
		"Outside the declared subset is Unmeasured, never a finding.",
		"An armed pre-commit rung names that limit and leaves it to the pipeline.",
		"Run golaw scan . before finishing.",
		"Run golaw scan --staged before finishing staged work.",
	)
	for _, forbidden := range []string{
		"when it exists",
		"Every developer must arm the rungs.",
		"Run make ci before every commit.",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("skill retains unsupported instruction: %s", forbidden)
		}
	}
}

const m3SkillPointer = "~/.agents/skills/golaw/SKILL.md"

func TestM3ProseBudgetBoundaries(t *testing.T) {
	t.Parallel()
	base := "agents/" + agentID(t) + "/"
	pointer := "Read " + m3SkillPointer + ".\n"
	skillLimit := "; limit 100 lines and 10000 bytes; shorten the skill."
	blockLimit := "; limit 1000 bytes with at least 1 pointer; shorten the block or add " + m3SkillPointer + "."
	ruleLimit := "; limit 20 lines; shorten the rule."
	rows := []struct {
		name, path, body, want string
	}{
		{"near skill lines", m3SkillPath, strings.Repeat("x\n", 100), ""},
		{"red skill lines", m3SkillPath, strings.Repeat("x\n", 101), "101 lines and 202 bytes" + skillLimit},
		{"near unterminated", m3SkillPath, strings.Repeat("x\n", 99) + "x", ""},
		{"red unterminated", m3SkillPath, strings.Repeat("x\n", 100) + "x", "101 lines and 201 bytes" + skillLimit},
		{"near skill bytes", m3SkillPath, strings.Repeat("x", 10000), ""},
		{"red skill bytes", m3SkillPath, strings.Repeat("x", 10001), "1 lines and 10001 bytes" + skillLimit},
		{"near UTF-8", m3SkillPath, strings.Repeat("é", 5000), ""},
		{"red UTF-8", m3SkillPath, strings.Repeat("é", 5000) + "x", "1 lines and 10001 bytes" + skillLimit},
		{"near copied skill", base + "skills/golaw/SKILL.md", strings.Repeat("x\n", 100), ""},
		{"red copied skill", base + "skills/golaw/SKILL.md", strings.Repeat("x\n", 101), "101 lines and 202 bytes" + skillLimit},
		{"red another skill", "agents/extra/SKILL.md", strings.Repeat("x", 10001), "1 lines and 10001 bytes" + skillLimit},
		{"near block bytes", base + "AGENTS.md", pointer + strings.Repeat("x", 1000-len(pointer)), ""},
		{"red block bytes", base + "AGENTS.md", pointer + strings.Repeat("x", 1001-len(pointer)), "1001 bytes and 1 skill pointers" + blockLimit},
		{"red missing pointer", base + "AGENTS.md", strings.Repeat("x", 1000), "1000 bytes and 0 skill pointers" + blockLimit},
		{"near rule lines", base + "golaw.mdc", strings.Repeat("x\n", 20), ""},
		{"red rule lines", base + "golaw.mdc", strings.Repeat("x\n", 21), "21 lines" + ruleLimit},
		{"near rule unterminated", base + "golaw.mdc", strings.Repeat("x\n", 19) + "x", ""},
		{"red rule unterminated", base + "golaw.mdc", strings.Repeat("x\n", 20) + "x", "21 lines" + ruleLimit},
		{"near CRLF", base + "golaw.mdc", strings.Repeat("x\r\n", 20), ""},
		{"red CRLF", base + "golaw.mdc", strings.Repeat("x\r\n", 21), "21 lines" + ruleLimit},
		{"near empty skill", m3SkillPath, "", ""},
		{"near on-demand prose", m3RulesPath, strings.Repeat("x\n", 301), ""},
		{"near root instructions", "AGENTS.md", strings.Repeat("x", 2000), ""},
		{"near adjacent directory", "agents-extra/SKILL.md", strings.Repeat("x", 10001), ""},
		{"near other asset", "agents/README.md", strings.Repeat("x", 10001), ""},
	}
	for _, row := range rows {
		want := row.want
		if want != "" {
			want = row.path + " has " + want
		}
		if got := inspectProseBudget(row.path, []byte(row.body)); got != want {
			t.Errorf("%s: reason=%q, want=%q", row.name, got, want)
		}
	}
}

func TestM3KitProseBudgets(t *testing.T) {
	t.Parallel()
	for _, path := range trackedPaths(t) {
		if !strings.HasPrefix(path, "agents/") {
			continue
		}
		if reason := inspectProseBudget(path, kitBytes(t, path)); reason != "" {
			t.Error(reason)
		}
	}
}

func inspectProseBudget(path string, data []byte) string {
	if !strings.HasPrefix(path, "agents/") {
		return ""
	}
	name, lines, size := filepath.Base(path), lineCount(data), len(data)
	switch {
	case name == "SKILL.md":
		if lines > 100 || size > 10000 {
			return fmt.Sprintf("%s has %d lines and %d bytes; limit 100 lines and 10000 bytes; shorten the skill.", path, lines, size)
		}
	case name == "AGENTS.md":
		pointers := bytes.Count(data, []byte(m3SkillPointer))
		if size > 1000 || pointers == 0 {
			return fmt.Sprintf("%s has %d bytes and %d skill pointers; limit 1000 bytes with at least 1 pointer; shorten the block or add %s.", path, size, pointers, m3SkillPointer)
		}
	case filepath.Ext(name) == ".mdc":
		if lines > 20 {
			return fmt.Sprintf("%s has %d lines; limit 20 lines; shorten the rule.", path, lines)
		}
	}
	return ""
}

func TestM3CatalogHasReasonsAndStableReferences(t *testing.T) {
	t.Parallel()
	rows := m3CatalogRows(t, kitBytes(t, m3RulesPath))
	for _, check := range checks.Catalog() {
		spec := check.Spec()
		row, found := rows[spec.ID]
		if !found {
			t.Errorf("missing catalog row: %s", spec.ID)
			continue
		}
		needs := strings.Join(spec.Needs, ",")
		if needs == "" {
			needs = "none"
		}
		if row[1] != spec.Rule || row[2] != needs {
			t.Errorf("%s documents rule or capability differently from its spec", spec.ID)
		}
		if !strings.Contains(strings.ToLower(row[3]), "because") {
			t.Errorf("%s has no stated why", spec.ID)
		}
		requireText(t, m3RulesPath, "## "+spec.Rule)
		delete(rows, spec.ID)
	}
	for id := range rows {
		t.Errorf("documented check is not registered: %s", id)
	}
}

func m3CatalogRows(t *testing.T, data []byte) map[string][]string {
	t.Helper()
	rows := make(map[string][]string)
	for line := range strings.SplitSeq(string(data), "\n") {
		cells := strings.Split(line, "|")
		if len(cells) != 6 {
			continue
		}
		id := strings.TrimSpace(cells[1])
		rule := strings.TrimSpace(cells[2])
		if !strings.HasPrefix(rule, "rules/") {
			continue
		}
		if !model.ValidID(id) {
			t.Fatalf("invalid documented check id: %s", id)
		}
		if _, found := rows[id]; found {
			t.Fatalf("duplicate documented check: %s", id)
		}
		rows[id] = []string{
			id, rule, strings.TrimSpace(cells[3]), strings.TrimSpace(cells[4]),
		}
	}
	return rows
}

func TestM3RequiredTargetsAreDocumentedFromTheirAuthority(t *testing.T) {
	t.Parallel()
	combinations := [][]string{
		{},
		{"image"},
		{"go"},
		{"go", "cli"},
		{"go", "api"},
		{"go", "db"},
		{"go", "image"},
		{"go", "hexagonal"},
		{"go", "api", "cli", "db", "hexagonal", "image"},
	}
	for _, capabilities := range combinations {
		key := strings.Join(capabilities, ",")
		if key == "" {
			key = "none"
		}
		targets := strings.Join(policyfiles.RequiredTargets(capabilities), " ")
		if targets == "" {
			targets = "none"
		}
		requireText(t, m3RulesPath, "| "+key+" | "+targets+" |")
	}
}

func TestM3ReaderRowsNameTheirLimits(t *testing.T) {
	t.Parallel()
	rows := []struct {
		name, file, limit, resolve string
	}{
		{"readGolangci", "lint_golangci.go", "a version other than \"2\" is outside the subset", "golangci-lint migrate"},
		{"readMakefile", "build_makefile.go", "a computed include name", "outside the conditional block"},
		{"readLefthook", "build_lefthook.go", "a second document is outside the subset", "one run string"},
		{"readActions", "build_actions.go", "directly under", "write the make target literally"},
		{"readModule", "deps_gomod.go", "every require and replace block included", "a path inside the repository"},
		{"readDockerfile", "deps_dockerfile.go", "an ARG without a default", "give the ARG a literal default"},
		{"readOwners", "deps_codeowners.go", "a negation or a bracket is outside the subset", "without a negation or a bracket"},
		{"readDependabot", "deps_dependabot.go", "a second document is outside the subset", "integer cooldown days"},
		{"decodeYAML", "yaml.go", "a root that is not a mapping or a second document is outside the subset", "one document whose root is a mapping"},
		{"decodeTOML", "formats.go", "a syntax error naming the file and the line", "fix the syntax at the line named"},
		{"decodeYAML (Redocly)", "yaml.go", "unknown inheritance, plugins or relevant assertion expressions", "keep references local"},
	}
	lines := strings.Split(string(kitBytes(t, m3RulesPath)), "\n")
	for _, row := range rows {
		prefix, count := "| "+row.name+" | "+row.file+" | ", 0
		for _, line := range lines {
			if strings.HasPrefix(line, prefix) && strings.Contains(line, row.limit) &&
				strings.Contains(line, row.resolve) && strings.Count(line, "|") == 6 {
				count++
			}
		}
		if count != 1 {
			t.Errorf("reader %s needs one row naming %s, limit %q and the rewrite %q", row.name, row.file, row.limit, row.resolve)
		}
	}
}

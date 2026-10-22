package cicheck_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

const (
	m3ChecklistsPath = "docs/checklists.md"
	m3ContextPath    = "docs/context-template.md"
)

func TestM3EngineeringGuides(t *testing.T) {
	t.Parallel()
	rows := []struct {
		name string
		path string
		text string
		want bool
	}{
		{"builder", m3ChecklistsPath, "## Builder", true},
		{"reviewer", m3ChecklistsPath, "## Reviewer", true},
		{"conditions", m3ChecklistsPath, "## Conditional checks", true},
		{"evidence", m3ChecklistsPath, "## Evidence", true},
		{"near local choice", m3ChecklistsPath, "Every local check is optional.", true},
		{"near remote evidence", m3ChecklistsPath, "Remote CI or the platform pipeline is the only gate.", true},
		{"red compulsory setup", m3ChecklistsPath, "Every developer must run golaw setup.", false},
		{"red heavy push", m3ChecklistsPath, "Run mutation testing in pre-push.", false},
		{"measurement", m3ChecklistsPath, "A check needs a red case, a near miss and an explicit input universe.", true},
		{"process results", m3ChecklistsPath, "Read both the exit code and stderr.", true},
		{"wall clock", m3ChecklistsPath, "Use benchmarks for wall-clock budgets.", true},
		{"equivalence", m3ChecklistsPath, "An equivalence claim names the applied input and identical observed result.", true},
		{"query plans", m3ChecklistsPath, "generate_series, ANALYZE and EXPLAIN", true},
		{"architecture evidence", m3ChecklistsPath, "GOARCH=amd64", true},
		{"one heavy process", m3ChecklistsPath, "Run only one heavy process at a time on the workstation.", true},
		{"transactions", m3ChecklistsPath, "Keep network calls outside transactions.", true},
		{"atomic money", m3ChecklistsPath, "Commit a money-path change as one database unit.", true},
		{"retry limits", m3ChecklistsPath, "Bound retries, provide a parking path, back off and expose permanent failure through liveness.", true},
		{"price constraints", m3ChecklistsPath, "Price constraints use CHECK, then NOT VALID and VALIDATE when expanding an existing table.", true},
		{"fingerprints", m3ChecklistsPath, "Store a version beside every persisted fingerprint.", true},
		{"retention", m3ChecklistsPath, "Every finite retention policy has a purge query.", true},
		{"database budgets", m3ChecklistsPath, "statement_timeout, lock_timeout and idle_in_transaction_session_timeout", true},
		{"idempotency", m3ChecklistsPath, "Test simultaneous duplicate delivery and the partial unique index.", true},
		{"contract identity", m3ChecklistsPath, "Expose the contract hash in liveness.", true},
		{"log identity", m3ChecklistsPath, "Use one version per process and log service, env, version and trace identifiers.", true},
		{"histograms", m3ChecklistsPath, "The highest finite histogram bucket covers the configured ceiling.", true},
		{"deadline budget", m3ChecklistsPath, "Budget deadlines from the request boundary to the database.", true},
		{"foreign conventions", m3ChecklistsPath, "Read the destination repository's branch, title and body conventions.", true},
		{"squash message", m3ChecklistsPath, "Check the author address and final squash message for an added trailer.", true},
		{"managed objects", m3ChecklistsPath, "Each managed object names its property, count, policy family, alternatives and reversal condition.", true},
		{"tool agreement", m3ChecklistsPath, "Two tools must not disagree silently.", true},
		{"recurrence", m3ChecklistsPath, "A repeated finding becomes a check when a meaningful red case and near miss exist.", true},
		{"lesson", m3ChecklistsPath, "Record a lesson the same day and date its change with a reason.", true},
		{"review ownership", m3ChecklistsPath, "Use tools for verification and one reviewer for judgment.", true},
		{"context boundaries", m3ContextPath, "## Boundaries", true},
		{"context shapes", m3ContextPath, "## Shapes", true},
		{"context tests", m3ContextPath, "## Tests", true},
		{"context order", m3ContextPath, "## Construction order", true},
		{"plain structs", m3ContextPath, "Use plain application input and output structs.", true},
		{"repository mapping", m3ContextPath, "Rows map to domain types in the repository.", true},
		{"handler signature", m3ContextPath, "Keep the generated strict handler signature.", true},
		{"clock", m3ContextPath, "Inject a Clock port and compose UTC at the entry point.", true},
		{"configuration", m3ContextPath, "Each configurable value has an env tag, a default, validation and documentation.", true},
		{"near no scaffolding", m3ContextPath, "This guide does not cause init to scaffold application source.", true},
	}
	for _, row := range rows {
		got := strings.Contains(string(kitBytes(t, row.path)), row.text)
		if got != row.want {
			t.Errorf("%s: content present=%v, want=%v", row.name, got, row.want)
		}
	}
}

func TestM3GuideLengths(t *testing.T) {
	t.Parallel()
	for _, path := range []string{m3ChecklistsPath, m3ContextPath} {
		if got := lineCount(kitBytes(t, path)); got > 300 {
			t.Errorf("%s has %d lines, maximum 300", path, got)
		}
	}
}

const m3ProfilesPath = "docs/profiles.md"

func TestM3ProfileContract(t *testing.T) {
	t.Parallel()
	requireText(t, m3ProfilesPath,
		"A profile is data, never code.",
		"A profile adds checks or tightens floors; it never relaxes a core rule.",
		"Template subtraction happens only during init.",
		"A profile's remote gate is whatever the adopting platform runs.",
		"Without a remote stage, automatic kit checks run only on machines whose developers armed them.",
		"A remote hard gate requires a demonstrated remote rejection.",
		"M3 validates the identity stamp and reference; payload loading and profile install belong to M5.",
		"Unsupported syntax or unavailable external evidence is Unmeasured, never a text match.",
		"Hooks never fetch a profile.",
		"An absent or mismatched profile never falls back to core.",
	)
	for _, key := range []string{"name", "version", "date", "source", "digest"} {
		requireText(t, m3ProfilesPath, "| `"+key+"` |")
	}
	if got := lineCount(kitBytes(t, m3ProfilesPath)); got > 300 {
		t.Fatalf("profile guide has %d lines, maximum 300", got)
	}
}

func TestM3ProfileExamplesUseTheExistingSchema(t *testing.T) {
	t.Parallel()
	stamp := string(m3ProfileYAML(t, "## Bundle identity"))
	rows := []struct {
		name string
		data string
		bad  bool
	}{
		{"near complete stamp", stamp, false},
		{"red unknown field", stamp + "unknown: true\n", true},
		{"red duplicate field", stamp + "name: second\n", true},
		{"red invalid date", strings.Replace(stamp, "2026-10-20", "2026-02-30", 1), true},
		{"red nonexact version", strings.Replace(stamp, "1.2.3", "latest", 1), true},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), policy.StampFile)
			if err := os.WriteFile(path, []byte(row.data), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := policy.LoadStamp(path)
			if errors.Is(err, model.ErrPolicy) != row.bad {
				t.Fatalf("stamp error=%v, invalid=%v", err, row.bad)
			}
		})
	}
	config, err := policy.Decode(m3ProfileYAML(t, "## Repository reference"))
	if err != nil {
		t.Fatal(err)
	}
	if err := config.Validate(nil); err != nil {
		t.Fatal(err)
	}
	if config.Profile == nil || config.Profile.Name != "fixture" {
		t.Fatal("repository example does not name its fixture profile")
	}
}

func m3ProfileYAML(t *testing.T, heading string) []byte {
	t.Helper()
	text := string(kitBytes(t, m3ProfilesPath))
	_, rest, found := strings.Cut(text, heading+"\n\n```yaml\n")
	if !found {
		t.Fatalf("profile example heading is absent: %s", heading)
	}
	body, _, found := strings.Cut(rest, "\n```")
	if !found {
		t.Fatalf("profile example is not closed: %s", heading)
	}
	return []byte(body + "\n")
}

func TestM3RulesStateMeasurementBoundaries(t *testing.T) {
	t.Parallel()
	requireText(t, m3RulesPath,
		"Policy-file checks are tree checks in scan and staged scan.",
		"They do not run in agent events or the commit-msg rung.",
		"An unsupported construct matters only when the verdict depends on it.",
		"Outside the declared subset is Unmeasured, never a finding.",
		"The reason names the file, line and construct.",
		"A required Unmeasured check makes scan exit 2.",
		"An armed pre-commit rung reports a declared-subset limit and leaves it to the pipeline.",
		"An unused Makefile variable does not widen the reader's scope.",
		"Makefile includes resolve from the repository root.",
		"An absent Makefile is named by policy.makefile alone.",
		"A stated limit names the construct, and docs/rules.md names the rewrite that clears it.",
		"An external local replace names its directive and leaves dependency measurement Unmeasured.",
		"policy.hook-tests and policy.codeowners-foreign require an enrolled profile.",
		"Without staged changes, policy.codeowners-foreign is NotApplicable.",
		"A missing lefthook.yml is not a finding.",
		"The push rung runs govulncheck only.",
		"scan remains the golaw scan; image-scan names the image scan.",
		"hooks-test is profile data, not a core required target.",
		"cli and hexagonal add no make target.",
		"commit.author-noreply is deferred: M2 never infers that a repository is public.",
		"M2 does not add a remote commit-range message gate.",
	)
	if got := lineCount(kitBytes(t, m3RulesPath)); got > 300 {
		t.Fatalf("rule catalog has %d lines, maximum 300", got)
	}
}

func TestM3CapabilitySetup(t *testing.T) {
	t.Parallel()
	rows := []struct {
		text string
		want bool
	}{
		{"## Capabilities and policy-file limits", true},
		{"api, cli, db and hexagonal each require go; image and go can stand alone.", true},
		{"Unknown or repeated capabilities are invalid.", true},
		{"golaw init --capabilities go,api,db,hexagonal", true},
		{"The preview writes nothing.", true},
		{"On an existing policy, the flag adds to the capabilities already enrolled.", true},
		{"An explicit capability proposal adds to the existing set and never removes a capability.", true},
		{"Apply writes the reviewed policy, matching enrollment and newly selected templates.", true},
		{"doctor.capabilities reports the capabilities in force.", true},
		{"With go off, coverage and Go rung steps are not applicable.", true},
		{"A disabled rung returns without a finding.", true},
		{"init never arms git hooks.", true},
		{"The pre-push rung runs govulncheck only.", true},
		{"M3 leaves migration, API-breaking and generation execution to their pipeline targets.", true},
		{"Outside the declared subset is Unmeasured, never a finding.", true},
		{"The pre-commit rung names that limit and continues unless another result refuses the commit.", true},
		{"Remote CI or the platform pipeline is the only hard gate.", true},
		{"Run make ci before every local commit.", false},
		{"Capabilities are inferred from the remote.", false},
	}
	text := string(kitBytes(t, "docs/setup.md"))
	for _, row := range rows {
		if got := strings.Contains(text, row.text); got != row.want {
			t.Errorf("setup fragment %q present=%v, want=%v", row.text, got, row.want)
		}
	}
	if got := lineCount([]byte(text)); got > 300 {
		t.Fatalf("setup guide has %d lines, maximum 300", got)
	}
}

func TestM3PipelineNoticeReplacesTemplateDeferrals(t *testing.T) {
	t.Parallel()
	for _, path := range []string{m3RulesPath, "docs/security.md"} {
		text := string(kitBytes(t, path))
		for _, stale := range []string{
			"Migration, API and generation rung tools arrive with the M3 templates.",
			"Migration, API and generation rung tools remain not applicable until the M3 templates.",
		} {
			if strings.Contains(text, stale) {
				t.Errorf("%s retains obsolete sentence %q", path, stale)
			}
		}
	}
	requireText(t, "docs/security.md",
		"The pre-commit rung leaves migration, API-breaking and generation execution to the pipeline targets.")
}

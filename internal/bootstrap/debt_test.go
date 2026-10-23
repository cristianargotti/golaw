package bootstrap_test

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// The record and the expiry every seeded entry of these tests carries.
const (
	debtRecord = "docs/adr/0002-inherited-debt.md"
	debtUntil  = "2027-04-12"
)

// dashedPath is a path with the dash the dash law refuses, spelled by code
// point so this file holds none.
func dashedPath() string { return "docs/a" + string(rune(0x2014)) + "b.md" }

func TestSeedDebtKeepsOneEntryPerEnforcedFindingWithAPath(t *testing.T) {
	t.Parallel()
	findings := []model.Finding{
		{Check: "text.todo", Path: "b.go", Severity: model.SeverityFail},
		{Check: "text.dashes", Path: "notes.md", Severity: model.SeverityDeny},
		{Check: "text.dashes", Path: "notes.md", Line: 9, Severity: model.SeverityDeny},
		{Check: "text.dashes", Path: "a.md", Severity: model.SeverityFail},
		{Check: "shell.no-verify", Path: "", Severity: model.SeverityDeny},
		{Check: "text.todo", Path: "c.go", Severity: model.SeverityNote},
	}
	got := bootstrap.SeedDebt(findings, []string{"z.go", "", "a.go", "z.go"}, []string{"GO-2026-0002", "GO-2026-0001", "GO-2026-0002"}, debtRecord, debtUntil)
	want := []policy.ExceptionRule{
		{Check: "text.dashes", Path: "a.md", Record: debtRecord, Until: debtUntil},
		{Check: "text.dashes", Path: "notes.md", Record: debtRecord, Until: debtUntil},
		{Check: "text.todo", Path: "b.go", Record: debtRecord, Until: debtUntil},
	}
	if !slices.Equal(got.Exceptions, want) {
		t.Errorf("exceptions = %+v", got.Exceptions)
	}
	if !slices.Equal(got.Exemptions, []string{"a.go", "z.go"}) {
		t.Errorf("exemptions = %v", got.Exemptions)
	}
	waivers := []policy.Waiver{{ID: "GO-2026-0001", Until: debtUntil, Record: debtRecord}, {ID: "GO-2026-0002", Until: debtUntil, Record: debtRecord}}
	if !slices.Equal(got.Waivers, waivers) {
		t.Errorf("waivers = %+v", got.Waivers)
	}
	if got.Empty() {
		t.Error("a seeded set reads empty")
	}
	if empty := bootstrap.SeedDebt(findings[4:], nil, nil, debtRecord, debtUntil); !empty.Empty() {
		t.Errorf("an event finding and a note were seeded: %+v", empty)
	}
}

func TestApplyDebtWritesTheThreeListsInThePolicyInitRendered(t *testing.T) {
	t.Parallel()
	d := bootstrap.Debt{
		Exceptions: []policy.ExceptionRule{
			{Check: "text.dashes", Path: dashedPath(), Record: debtRecord, Until: debtUntil},
			{Check: "text.todo", Path: "dir with space/b: c.go", Record: debtRecord, Until: debtUntil},
		},
		Exemptions: []string{"internal/a.go", "internal/b #1.go"},
		Waivers:    []policy.Waiver{{ID: "GO-2026-0001", Until: debtUntil, Record: debtRecord}},
	}
	got, err := bootstrap.ApplyDebt([]byte(renderedCore), d)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.ContainsRune(got, 0x2014) || !bytes.Contains(got, []byte(`\u2014`)) {
		t.Errorf("the dash of a path was written as the rune:\n%s", got)
	}
	f, err := policy.Decode(got)
	if err != nil {
		t.Fatalf("decode:\n%s\n%v", got, err)
	}
	if !slices.Equal(f.Exceptions, d.Exceptions) || !slices.Equal(f.Waivers, d.Waivers) || !slices.Equal(f.Checks.Coverage.FileExemptions, d.Exemptions) {
		t.Errorf("decoded = %+v %+v %+v", f.Exceptions, f.Waivers, f.Checks.Coverage.FileExemptions)
	}
	if err := f.Validate(nil); err != nil {
		t.Errorf("the seeded policy does not validate: %v", err)
	}
	for line := range strings.SplitSeq(renderedCore, "\n") {
		edited := strings.Contains(line, "file_exemptions") || line == "vulnerability_waivers: []" || line == "exceptions: []"
		if !edited && !strings.Contains(string(got), line+"\n") {
			t.Errorf("a line init wrote is gone: %q", line)
		}
	}
	if want := `file_exemptions: ["internal/a.go", "internal/b #1.go"] }`; !strings.Contains(string(got), want) {
		t.Errorf("the coverage line lacks %q:\n%s", want, got)
	}
	if same, err := bootstrap.ApplyDebt([]byte(renderedCore), bootstrap.Debt{}); err != nil || string(same) != renderedCore {
		t.Errorf("an empty set changed the policy: %v", err)
	}
}

func TestApplyDebtRefusesAPolicyInitDidNotWrite(t *testing.T) {
	t.Parallel()
	exception := bootstrap.Debt{Exceptions: []policy.ExceptionRule{{Check: "text.todo", Path: "a.go", Record: debtRecord, Until: debtUntil}}}
	exemption := bootstrap.Debt{Exemptions: []string{"a.go"}}
	waiver := bootstrap.Debt{Waivers: []policy.Waiver{{ID: "GO-2026-0001", Until: debtUntil, Record: debtRecord}}}
	for _, tc := range []struct {
		name, current string
		debt          bootstrap.Debt
		want          string
		usage         bool
	}{
		{"red block exceptions", strings.Replace(renderedCore, "exceptions: []", "exceptions:\n  - { check: text.todo, path: z.go, record: r.md, until: 2027-01-01 }", 1), exception, "exceptions", true},
		{"red no coverage line", strings.Replace(renderedCore, ", file_exemptions: []", "", 1), exemption, "file_exemptions", true},
		{"red waivers held", strings.Replace(renderedCore, "vulnerability_waivers: []", "vulnerability_waivers:\n  - { id: GO-2026-0009, until: 2027-01-01, record: r.md }", 1), waiver, "vulnerability_waivers", true},
		{"red not a policy", "capabilities: [go\n", exception, "read golaw.yml before the debt set", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := bootstrap.ApplyDebt([]byte(tc.current), tc.debt)
			if err == nil || got != nil || !strings.Contains(err.Error(), tc.want) || errors.Is(err, model.ErrUsage) != tc.usage {
				t.Fatalf("ApplyDebt = %q, %v", got, err)
			}
		})
	}
}

func TestDebtRecordListsEveryEntryInTheLanguageOfThePolicy(t *testing.T) {
	t.Parallel()
	d := bootstrap.Debt{
		Exceptions: []policy.ExceptionRule{{Check: "text.dashes", Path: dashedPath(), Record: debtRecord, Until: debtUntil}, {Check: "text.todo", Path: "a|b.go", Record: debtRecord, Until: debtUntil}},
		Exemptions: []string{"internal/a.go"},
		Waivers:    []policy.Waiver{{ID: "GO-2026-0001", Until: debtUntil, Record: debtRecord}},
	}
	portuguese := string(bootstrap.DebtRecord(d, "2026-10-14", model.DefaultLanguage))
	for _, want := range []string{
		"---\nrecord: 1\ntitle: inherited debt\nstatus: inherited\n---\n", "# Dívida herdada", "2026-10-14",
		`| text.dashes | "docs/a\u2014b.md" | 2027-04-12 |`, `| text.todo | "a\|b.go" | 2027-04-12 |`,
		`| "internal/a.go" |`, "| GO-2026-0001 | 2027-04-12 |", "golaw init --apply --seed-debt",
	} {
		if !strings.Contains(portuguese, want) {
			t.Errorf("the record lacks %q:\n%s", want, portuguese)
		}
	}
	if strings.ContainsRune(portuguese, 0x2014) {
		t.Errorf("the record holds the dash of a path as the rune:\n%s", portuguese)
	}
	english := string(bootstrap.DebtRecord(d, "2026-10-14", "en"))
	if !strings.Contains(english, "# Inherited debt") || strings.Contains(english, "Dívida") || !strings.Contains(english, `| text.todo | "a\|b.go" | 2027-04-12 |`) {
		t.Errorf("the English record:\n%s", english)
	}
	if only := string(bootstrap.DebtRecord(bootstrap.Debt{Waivers: d.Waivers}, "2026-10-14", "en")); strings.Contains(only, "| Check |") || strings.Contains(only, "| Coverage exemption |") {
		t.Errorf("an empty table was written:\n%s", only)
	}
}

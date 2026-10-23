package main

import (
	"path/filepath"
	"strings"
	"testing"
)

const m5Limitation = "A profile's remote gate is whatever the adopting platform runs. " +
	"Without a remote stage, automatic kit checks run only on machines whose developers armed them.\n"

func TestM5RemoteOutcomeNamesWhichHolds(t *testing.T) {
	t.Parallel()
	record := `{"date": "2026-11-02", "owner": "kit owner", "platform": "the adopting platform's pipeline", ` +
		`"revision": "0123456789abcdef0123456789abcdef01234567", "violation": "a typographic dash in a Go file", ` +
		`"rejection": "the pipeline stage failed the merge request"}`
	rows := []struct {
		name, page, record, detail string
		pass                       bool
	}{
		{"near the limitation stated", m5Limitation, "", "generic limitation", true},
		{"near a rejection recorded", m5Limitation, record, "remote rejection recorded on 2026-11-02", true},
		{"red neither", "# Profiles\n", "", "", false},
		{"red half the statement", strings.SplitAfter(m5Limitation, ". ")[0], "", "", false},
		{"red unknown field", m5Limitation, strings.Replace(record, `"date"`, `"when": 1, "date"`, 1), "", false},
		{"red no rejection", m5Limitation, strings.Replace(record, "the pipeline stage failed the merge request", "", 1), "", false},
		{"red short revision", m5Limitation, strings.Replace(record, "0123456789abcdef0123456789abcdef01234567", "0123abc", 1), "", false},
		{"red no date", m5Limitation, strings.Replace(record, "2026-11-02", "soon", 1), "", false},
		{"red not JSON", m5Limitation, "{", "", false},
	}
	for _, row := range rows {
		root := t.TempDir()
		checkedWrite(t, filepath.Join(root, "docs", "profiles.md"), row.page)
		if row.record != "" {
			checkedWrite(t, filepath.Join(root, "docs", "live", "remote-gate.json"), row.record)
		}
		detail, err := m5RemoteOutcome(root)
		if (err == nil) != row.pass || !strings.Contains(detail, row.detail) {
			t.Errorf("%s: detail %q, %v", row.name, detail, err)
		}
	}
}

func TestM5RemoteRowRunsOverTheKit(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	rows, _ := s.m5CollectCatalogs(t.Context(), []m5Catalog{{category: m5RemoteCategory, cases: m5RemoteCases()}})
	if len(rows) != 1 || rows[0].name != "gate" || rows[0].status != statusPass || !rows[0].required ||
		(!strings.Contains(rows[0].detail, "generic limitation") && !strings.Contains(rows[0].detail, "remote rejection recorded")) {
		t.Fatalf("rows = %+v", rows)
	}
	if err := m5RemoteCases()[0].run(s, t.Context()); err != nil {
		t.Fatal(err)
	}
}

// TestM5CatalogsRequireTheRemoteGateRow holds the wiring that makes -m5
// and -release run the row: the remote category is one of the M5
// catalogs, with its one case.
func TestM5CatalogsRequireTheRemoteGateRow(t *testing.T) {
	t.Parallel()
	for _, catalog := range m5Catalogs() {
		if catalog.category != m5RemoteCategory {
			continue
		}
		if len(catalog.cases) != 1 || catalog.cases[0].name != "gate" {
			t.Fatalf("the remote catalog = %+v", catalog.cases)
		}
		return
	}
	t.Fatal("no M5 catalog carries the remote gate row")
}

package main

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	scanreport "github.com/cristianargotti/golaw/internal/report"
)

// m5RedDocument is the scan of the red tree under the profile: one finding
// per evaluator and nothing else.
func m5RedDocument() scanreport.Document {
	doc := scanreport.Document{Schema: 1}
	for _, check := range m5Evaluators() {
		doc.Findings = append(doc.Findings, model.Finding{Check: check, Path: "Dockerfile", Code: "red", Message: "red", Severity: model.SeverityDeny})
	}
	return doc
}

// m5StatusDocument is a clean scan whose profile checks all answer kind.
func m5StatusDocument(kind model.StatusKind) scanreport.Document {
	doc := scanreport.Document{Schema: 1}
	for _, check := range m5Evaluators() {
		doc.Checks = append(doc.Checks, scanreport.Status{ID: check, Status: kind.String()})
	}
	return doc
}

// m5ScanReply answers scan with doc and code, after requiring the files
// the tree must and must not hold when the scan runs.
func m5ScanReply(t *testing.T, doc scanreport.Document, code int, present, absent []string) m5Reply {
	t.Helper()
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return m5Reply{prefix: []string{commandScan}, once: true, answer: func(t *testing.T, c command) result {
		t.Helper()
		for _, path := range present {
			if _, err := os.Stat(filepath.Join(c.dir, filepath.FromSlash(path))); err != nil {
				t.Errorf("%s is absent at the scan: %v", path, err)
			}
		}
		for _, path := range absent {
			if _, err := os.Stat(filepath.Join(c.dir, filepath.FromSlash(path))); err == nil {
				t.Errorf("%s is present at the scan", path)
			}
		}
		return result{code: code, stdout: string(body)}
	}}
}

// m5ProfileReplies are the install, init and scan answers of a profile
// row, the scans given in order.
func m5ProfileReplies(t *testing.T, scans ...m5Reply) []m5Reply {
	t.Helper()
	replies := make([]m5Reply, 0, 2+len(scans))
	replies = append(replies,
		m5Reply{prefix: []string{m5ProfileCommand, m5InstallSub}, answer: m5After(m5FakeInstall(m5Version), result{stdout: "Perfil fixture 1.2.3 instalado\n"})},
		m5Reply{prefix: []string{commandInit}, answer: m5After(m5FakeInit, result{})},
	)
	return append(replies, scans...)
}

func TestM5JudgesOfTheFixtureTree(t *testing.T) {
	t.Parallel()
	missing := m5RedDocument()
	missing.Findings = missing.Findings[:len(missing.Findings)-1]
	extra := m5RedDocument()
	extra.Findings = append(extra.Findings, model.Finding{Check: "text.dashes", Severity: model.SeverityDeny})
	unmeasured := m5RedDocument()
	unmeasured.Counts.Unmeasured = 1
	measuredCore := m5StatusDocument(model.StatusNotApplicable)
	measuredCore.Checks[0].Status = model.StatusMeasured.String()
	finding := m5StatusDocument(model.StatusMeasured)
	finding.Findings = m5RedDocument().Findings[:1]
	rows := []struct {
		name  string
		judge func(scanreport.Document, int) error
		doc   scanreport.Document
		code  int
		pass  bool
	}{
		{"near profiled", m5JudgeProfiled, m5RedDocument(), 1, true},
		{"red profiled without the term", m5JudgeProfiled, missing, 1, false},
		{"red profiled with a core finding", m5JudgeProfiled, extra, 1, false},
		{"red profiled unmeasured", m5JudgeProfiled, unmeasured, 1, false},
		{"red profiled exit 2", m5JudgeProfiled, m5RedDocument(), 2, false},
		{"near core only", m5JudgeCore, m5StatusDocument(model.StatusNotApplicable), 0, true},
		{"red core measured a profile check", m5JudgeCore, measuredCore, 0, false},
		{"red core finding", m5JudgeCore, m5RedDocument(), 1, false},
		{"near corrected", m5JudgeCorrected, m5StatusDocument(model.StatusMeasured), 0, true},
		{"red corrected not applicable", m5JudgeCorrected, m5StatusDocument(model.StatusNotApplicable), 0, false},
		{"red corrected finding", m5JudgeCorrected, finding, 1, false},
	}
	for _, row := range rows {
		if err := row.judge(row.doc, row.code); (err == nil) != row.pass {
			t.Errorf("%s: %v", row.name, err)
		}
	}
	if got := m5FindingChecks(extra); !slices.Contains(got, "text.dashes") || !slices.IsSorted(got) || len(got) != len(m5Evaluators())+1 {
		t.Fatalf("finding checks = %q", got)
	}
}

func TestM5FixtureTreesHoldEveryViolationAndItsCorrection(t *testing.T) {
	t.Parallel()
	red, err := m5Files(testRoot(t), m5Red)
	if err != nil {
		t.Fatal(err)
	}
	corrected, err := m5Files(testRoot(t), m5Corrected)
	if err != nil {
		t.Fatal(err)
	}
	for path, fragments := range map[string][]string{
		"Dockerfile":         {"docker.example.test/", "curl=", "FROM registry.example.test/library/alpine@sha256:c"},
		m5RedOnly:            {"agents"},
		"deploy/values.yaml": {"runAsNonRoot: false"},
		"ci/pipeline.md":     {"legacyci"},
	} {
		for _, fragment := range fragments {
			if !strings.Contains(string(red[path]), fragment) {
				t.Errorf("red %s lacks %q", path, fragment)
			}
		}
	}
	if _, found := corrected[m5RedOnly]; found || !strings.Contains(string(corrected["Dockerfile"]), "\nFROM scratch\n") ||
		!strings.Contains(string(corrected["Dockerfile"]), "\nUSER 65532:65532\n") ||
		!strings.Contains(string(corrected["Makefile"]), "\nci: hooks-test image-lint\n") ||
		strings.Contains(string(corrected["ci/pipeline.md"]), "legacyci") || strings.Contains(string(corrected["Dockerfile"]), "curl") {
		t.Fatalf("corrected tree = %q", slices.Sorted(maps.Keys(corrected)))
	}
	root := t.TempDir()
	checkedWrite(t, filepath.Join(root, "tree", "plain"), "no suffix")
	if _, err := m5Files(root, "tree"); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("a file without its suffix = %v", err)
	}
	if err := m5Overlay(root, "absent", t.TempDir()); err == nil {
		t.Fatal("a missing fixture directory was written")
	}
}

func TestM5ProfileRowsWithASimulatedCandidate(t *testing.T) {
	t.Parallel()
	redFiles := []string{"Dockerfile", m5RedOnly, "deploy/values.yaml", "ci/pipeline.md"}
	rows := []struct {
		name  string
		scans func(t *testing.T) []m5Reply
	}{
		{"fixture-fails-with-profile", func(t *testing.T) []m5Reply {
			t.Helper()
			return []m5Reply{m5ScanReply(t, m5RedDocument(), 1, redFiles, nil)}
		}},
		{"fixture-passes-without", func(t *testing.T) []m5Reply {
			t.Helper()
			return []m5Reply{m5ScanReply(t, m5StatusDocument(model.StatusNotApplicable), 0, redFiles, nil)}
		}},
		{"fixture-passes-corrected", func(t *testing.T) []m5Reply {
			t.Helper()
			return []m5Reply{m5ScanReply(t, m5StatusDocument(model.StatusMeasured), 0, []string{"Dockerfile", "Makefile"}, []string{m5RedOnly})}
		}},
	}
	for _, row := range rows {
		c := m5ProfileCases()[slices.IndexFunc(m5ProfileCases(), func(c m5Case) bool { return c.name == row.name })]
		s := m2TestSuite(t)
		scope := s.m5Scope(m5ProfileCategory, row.name)
		scope.execute = m5Script(t, scope, m5ProfileReplies(t, row.scans(t)...)...).Execute
		if err := c.run(scope, t.Context()); err != nil {
			t.Fatalf("%s: %v", row.name, err)
		}
		failing := s.m5Scope(m5ProfileCategory, row.name+"-red")
		failing.execute = m5Script(t, failing, m5ProfileReplies(t, m5ScanReply(t, m5RedDocument(), 2, nil, nil))...).Execute
		if err := c.run(failing, t.Context()); err == nil {
			t.Fatalf("%s passed on a scan that exited 2", row.name)
		}
	}
	if names := m5Names(m5ProfileCases()); !slices.Equal(names[:3], []string{"fixture-fails-with-profile", "fixture-passes-without", "fixture-passes-corrected"}) {
		t.Fatalf("names = %q", names)
	}
	s := m2TestSuite(t)
	clean := s.m5Scope(m5ProfileCategory, "clean")
	clean.execute = m5Script(t, clean, m5ProfileReplies(t)...).Execute
	setup, err := clean.m5CleanRepository(t.Context(), "clean")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(setup.installed.stdout, "instalado") || len(setup.pin) == 0 || setup.source == "" {
		t.Fatalf("setup = %+v", setup)
	}
	if _, err := os.Stat(filepath.Join(setup.repo, "Makefile")); err != nil {
		t.Fatalf("the corrected tree is absent: %v", err)
	}
	m5Clean(t, clean, setup.repo)
}

func TestM5ProfileRowsReportEveryFailedStep(t *testing.T) {
	t.Parallel()
	scans := map[string]func(t *testing.T) m5Reply{
		"fixture-fails-with-profile": func(t *testing.T) m5Reply {
			t.Helper()
			return m5ScanReply(t, m5RedDocument(), 1, nil, nil)
		},
		"fixture-passes-without": func(t *testing.T) m5Reply {
			t.Helper()
			return m5ScanReply(t, m5StatusDocument(model.StatusNotApplicable), 0, nil, nil)
		},
		"fixture-passes-corrected": func(t *testing.T) m5Reply {
			t.Helper()
			return m5ScanReply(t, m5StatusDocument(model.StatusMeasured), 0, nil, nil)
		},
	}
	for _, c := range m5ProfileCases()[:3] {
		m5Sweep(t, m5ProfileCategory, c, func(t *testing.T, _ *suite) []m5Reply {
			t.Helper()
			return m5ProfileReplies(t, scans[c.name](t))
		})
	}
}

// TestM5CatalogsRequireTheProfileRows holds the wiring that makes -m5 and
// -release require the profile rows, which the row tests above reach only
// through m5ProfileCases.
func TestM5CatalogsRequireTheProfileRows(t *testing.T) {
	t.Parallel()
	for _, catalog := range m5Catalogs() {
		if catalog.category == m5ProfileCategory {
			if !slices.Equal(m5Names(catalog.cases), m5Names(m5ProfileCases())) {
				t.Fatalf("the profile catalog = %q", m5Names(catalog.cases))
			}
			return
		}
	}
	t.Fatal("no M5 catalog carries the profile rows")
}

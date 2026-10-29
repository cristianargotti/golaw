package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	scanreport "github.com/cristianargotti/golaw/internal/report"
	"github.com/cristianargotti/golaw/internal/surfacefix"
)

// m6PolicySample is the policy the simulated init writes: the ladder line
// init --apply renders today, which m6WithoutFeedback edits.
const m6PolicySample = "golaw: 1\nkit: { version: 0.6.0 }\ncapabilities: [go, image]\n" +
	"ladder: { pre_commit: true, commit_msg: true, pre_push: true, test_staged: false }\n"

// m6TestSuite is an M6 suite over the simulated candidate of M2.
func m6TestSuite(t *testing.T) *m6Suite {
	t.Helper()
	return newM6Suite(m2TestSuite(t))
}

// m6FakeInit stands in for init --apply: the sample policy in the checkout.
func m6FakeInit(t *testing.T, c command) {
	t.Helper()
	m2Write(t, filepath.Join(c.dir, policyFile), m6PolicySample)
}

// m6InitReply answers every init of the candidate with the sample policy.
func m6InitReply() m5Reply {
	return m5Reply{prefix: []string{commandInit}, answer: m5After(m6FakeInit, result{})}
}

// m6Doc is the JSON document scan --json or lint --json prints.
func m6Doc(t *testing.T, findings []model.Finding, checks ...scanreport.Status) string {
	t.Helper()
	doc := scanreport.Document{Schema: scanreport.Schema, Mode: "scan", Verdict: "clean", Findings: findings, Checks: checks}
	if len(findings) > 0 {
		doc.Verdict = "fail"
	}
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(body) + "\n"
}

// m6Found are the findings that meet the expectations of one source: the
// lint tools when lint, the native checks otherwise.
func m6Found(exps []surfacefix.Expectation, lint bool) []model.Finding {
	var out []model.Finding
	for _, e := range exps {
		if strings.HasPrefix(e.Check, m6LintPrefix) != lint {
			continue
		}
		for range e.Count {
			out = append(out, model.Finding{Check: e.Check, Path: e.Path, Code: e.Code, Line: 1, Severity: model.SeverityFail})
		}
	}
	return out
}

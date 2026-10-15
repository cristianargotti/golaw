// Package report renders an engine decision for people and machines: the
// text lines of a scan, the JSON document with its digests and counts, the
// fingerprint multiset staged mode compares, and the sentence of the human
// channel in the configured language. Findings stay English; only the human
// channel is translated, and no rendering ever carries a secret value.
package report

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// Schema is the version of the JSON document.
const Schema = 1

// Identity names the run a document describes beyond the decision.
type Identity struct {
	KitVersion   string // the version of the binary
	PolicyDigest string // the digest of the policy in force
	Ruleset      string // the digest of the registry, from RulesetDigest
	Snapshot     string // the snapshot identity the caller names: kind and revision
}

// Status is the measurement status of one check.
type Status struct {
	ID     string `json:"id"`
	Rule   string `json:"rule"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	Scope  string `json:"scope,omitempty"`
}

// Counts summarizes a document.
type Counts struct {
	Findings   int `json:"findings"`   // every finding, notes included
	Enforced   int `json:"enforced"`   // findings that count against the run
	Notes      int `json:"notes"`      // findings reported, never enforced
	Excluded   int `json:"excluded"`   // checks not applicable to the run
	Unmeasured int `json:"unmeasured"` // required checks that could not run
}

// Document is the JSON report of one run; its field order is its key order.
type Document struct {
	Schema     int             `json:"schema"`
	KitVersion string          `json:"kit_version"`
	Mode       string          `json:"mode"`
	Verdict    string          `json:"verdict"`
	Reason     string          `json:"reason,omitempty"`
	Policy     string          `json:"policy_digest"`
	Ruleset    string          `json:"ruleset_digest"`
	Snapshot   string          `json:"snapshot"`
	Findings   []model.Finding `json:"findings"`
	Checks     []Status        `json:"checks"`
	Counts     Counts          `json:"counts"`
}

// New builds the document of a run: the decision's findings in their sorted
// order, one status per outcome in registry order, and the counts.
func New(run *engine.Run, decision engine.Decision, id Identity) Document {
	doc := Document{
		Schema: Schema, KitVersion: id.KitVersion, Mode: decision.Mode.String(), Verdict: decision.Verdict.String(),
		Reason: decision.Reason, Policy: id.PolicyDigest, Ruleset: id.Ruleset, Snapshot: id.Snapshot,
		Findings: slices.Clone(decision.Findings), Checks: statuses(run),
	}
	if doc.Findings == nil {
		doc.Findings = []model.Finding{}
	}
	doc.Counts = count(doc)
	return doc
}

// statuses lists the measurement status of every outcome of the run.
func statuses(run *engine.Run) []Status {
	if run == nil {
		return []Status{}
	}
	out := make([]Status, 0, len(run.Outcomes))
	for i := range run.Outcomes {
		o := &run.Outcomes[i]
		s := o.Result.Status
		out = append(out, Status{ID: o.Spec.ID, Rule: o.Spec.Rule, Status: s.Kind.String(), Reason: s.Reason, Scope: o.Result.Scope})
	}
	return out
}

// count summarizes the findings and the statuses of a document.
func count(doc Document) Counts {
	c := Counts{Findings: len(doc.Findings)}
	for _, f := range doc.Findings {
		if f.Enforced() {
			c.Enforced++
		} else {
			c.Notes++
		}
	}
	for _, s := range doc.Checks {
		switch s.Status {
		case model.StatusNotApplicable.String():
			c.Excluded++
		case model.StatusUnmeasured.String():
			c.Unmeasured++
		}
	}
	return c
}

// RulesetDigest identifies the rules a registry enforces: the sha256 of
// every check id with its rule reference, in id order, one per line.
func RulesetDigest(checks []model.Check) string {
	lines := make([]string, 0, len(checks))
	for _, c := range checks {
		s := c.Spec()
		lines = append(lines, s.ID+" "+s.Rule+"\n")
	}
	slices.Sort(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "")))
	return hex.EncodeToString(sum[:])
}

// Multiset counts the findings by fingerprint, so two runs compare across
// line shifts.
func Multiset(findings []model.Finding) map[string]int {
	out := make(map[string]int, len(findings))
	for _, f := range findings {
		out[f.Fingerprint]++
	}
	return out
}

// Introduced returns the findings of a candidate run that a base run does
// not carry, counted as a multiset: a second identical occurrence is new
// when the base holds one. The order of the candidate is kept.
func Introduced(candidate, base []model.Finding) []model.Finding {
	left := Multiset(base)
	var out []model.Finding
	for _, f := range candidate {
		if left[f.Fingerprint] > 0 {
			left[f.Fingerprint]--
			continue
		}
		out = append(out, f)
	}
	return out
}

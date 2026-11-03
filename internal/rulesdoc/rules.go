// Package rulesdoc reads the rule tables of docs/rules.md and the sections
// of docs/checklists.md from the bytes the binary embeds, so an agent reads
// one rule or one checklist locally instead of fetching the pages.
package rulesdoc

import (
	"fmt"
	"strings"
)

// Rule is one row of a rule table: the stable check id, the rule
// reference with its version, the capability it needs and its why.
type Rule struct {
	ID, Ref, Needs, Why string
}

// rowCells is the column count of a rule table row.
const rowCells = 4

// summaryRunes bounds an index line, so the index stays one screen.
const summaryRunes = 90

// ParseRules returns the rule rows in page order; a row is a table line
// whose second cell is a rule reference, which skips headers, the prose
// mapping and the reader table.
func ParseRules(data []byte) []Rule {
	var rules []Rule
	for line := range strings.SplitSeq(string(data), "\n") {
		if rule, ok := parseRow(strings.TrimRight(line, "\r")); ok {
			rules = append(rules, rule)
		}
	}
	return rules
}

// parseRow splits one table line; the why cell keeps any later pipe.
func parseRow(line string) (Rule, bool) {
	if !strings.HasPrefix(line, "| ") || !strings.HasSuffix(line, " |") {
		return Rule{}, false
	}
	cells := strings.SplitN(strings.TrimSuffix(strings.TrimPrefix(line, "| "), " |"), " | ", rowCells)
	if len(cells) != rowCells || !strings.HasPrefix(cells[1], "rules/") {
		return Rule{}, false
	}
	return Rule{ID: cells[0], Ref: cells[1], Needs: cells[2], Why: cells[3]}, true
}

// Select keeps the rules whose id or reference starts with the query, so
// a family (goast), an id (commit.shape) or a section (rules/4.12) works.
func Select(rules []Rule, query string) ([]Rule, error) {
	var kept []Rule
	for _, rule := range rules {
		if strings.HasPrefix(rule.ID, query) || strings.HasPrefix(rule.Ref, query) {
			kept = append(kept, rule)
		}
	}
	if len(kept) == 0 {
		return nil, fmt.Errorf("%w %q; golaw rules lists every id", ErrNoRule, query)
	}
	return kept, nil
}

// Line is the full form of a rule, one line with every column.
func (r Rule) Line() string {
	return r.ID + " " + r.Ref + " needs " + r.Needs + ": " + r.Why
}

// IndexLine is the short form: the id and what it denies, cut before its
// reason and bounded so the whole index stays compact.
func (r Rule) IndexLine() string {
	what, _, _ := strings.Cut(r.Why, ", because")
	what = strings.TrimSuffix(what, ".")
	if runes := []rune(what); len(runes) > summaryRunes {
		what = strings.TrimSpace(string(runes[:summaryRunes])) + "..."
	}
	return r.ID + ": " + what
}

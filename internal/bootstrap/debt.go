package bootstrap

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// Debt is the reviewed debt set init --apply --seed-debt writes once (A24):
// what the repository already breaks on the day of its adoption, each entry
// with the record that names it and the day it expires.
type Debt struct {
	Exceptions []policy.ExceptionRule
	Exemptions []string
	Waivers    []policy.Waiver
}

// The three empty lists of the policy init renders, the only form
// ApplyDebt edits; the coverage list sits inside the flow mapping of its
// line, so it takes a flow sequence where the others take a block list.
var (
	waiverLine    = regexp.MustCompile(`(?m)^vulnerability_waivers:[ \t]*\[[ \t]*\][ \t]*$`)
	exceptionLine = regexp.MustCompile(`(?m)^exceptions:[ \t]*\[[ \t]*\][ \t]*$`)
	exemptionList = regexp.MustCompile(`(?m)^([ \t]+coverage:[ \t]*\{[^}\n]*file_exemptions:[ \t]*)\[[ \t]*\]`)
)

// SeedDebt builds the set from what a scan found and the measurements the
// person gave: one exception per enforced finding that names a path,
// sorted by check then path, once per pair; the coverage files under the
// floor and the reachable advisory ids, sorted and once each. Event
// findings carry no path and notes never count, so neither is seeded.
func SeedDebt(findings []model.Finding, exemptions, advisories []string, record, until string) Debt {
	d := Debt{Exemptions: sortedSet(exemptions)}
	seen := make(map[[2]string]bool, len(findings))
	for _, f := range findings {
		key := [2]string{f.Check, f.Path}
		if f.Path == "" || !f.Enforced() || seen[key] {
			continue
		}
		seen[key] = true
		d.Exceptions = append(d.Exceptions, policy.ExceptionRule{Check: f.Check, Path: f.Path, Record: record, Until: until})
	}
	slices.SortFunc(d.Exceptions, func(a, b policy.ExceptionRule) int {
		return cmp.Or(cmp.Compare(a.Check, b.Check), cmp.Compare(a.Path, b.Path))
	})
	for _, id := range sortedSet(advisories) {
		d.Waivers = append(d.Waivers, policy.Waiver{ID: id, Until: until, Record: record})
	}
	return d
}

// Empty reports whether the set holds no entry at all.
func (d Debt) Empty() bool {
	return len(d.Exceptions) == 0 && len(d.Exemptions) == 0 && len(d.Waivers) == 0
}

// ApplyDebt writes the three lists into the policy init rendered: the empty
// flow lines of vulnerability_waivers and exceptions become block lists and
// the empty file_exemptions of the coverage line a flow sequence. Every
// value is a double-quoted ASCII scalar, so a path keeps its bytes and the
// file holds no character a text rule judges. Any other form is a usage
// error naming the key, and the result is decoded again before it is
// answered; an empty set answers the bytes unchanged.
func ApplyDebt(current []byte, d Debt) ([]byte, error) {
	if d.Empty() {
		return current, nil
	}
	if _, err := policy.Decode(current); err != nil {
		return nil, fmt.Errorf("read %s before the debt set: %w", PolicyFile, err)
	}
	out := string(current)
	var err error
	for _, edit := range []struct {
		key     string
		pattern *regexp.Regexp
		value   string
		used    bool
	}{
		{"vulnerability_waivers", waiverLine, "vulnerability_waivers:\n" + waiverLines(d.Waivers), len(d.Waivers) > 0},
		{"exceptions", exceptionLine, "exceptions:\n" + exceptionLines(d.Exceptions), len(d.Exceptions) > 0},
		{"checks.coverage.file_exemptions", exemptionList, "${1}" + flowList(d.Exemptions), len(d.Exemptions) > 0},
	} {
		if !edit.used {
			continue
		}
		if out, err = replaceOnce(out, edit.key, edit.pattern, edit.value); err != nil {
			return nil, err
		}
	}
	return verifyDebt([]byte(out), d)
}

// replaceOnce replaces the one match of an empty list, or names the key
// when the policy holds it in another form.
func replaceOnce(text, key string, pattern *regexp.Regexp, value string) (string, error) {
	if len(pattern.FindAllStringIndex(text, -1)) != 1 {
		return "", fmt.Errorf("%w: %s holds %s in a form init does not edit; the debt set is seeded only into the policy init renders",
			model.ErrUsage, PolicyFile, key)
	}
	return pattern.ReplaceAllString(text, value), nil
}

// verifyDebt decodes the edited policy and compares the three lists.
func verifyDebt(data []byte, d Debt) ([]byte, error) {
	f, err := policy.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("decode the seeded %s: %w", PolicyFile, err)
	}
	var exemptions []string
	if f.Checks.Coverage != nil {
		exemptions = f.Checks.Coverage.FileExemptions
	}
	if !slices.Equal(f.Exceptions, d.Exceptions) || !slices.Equal(f.Waivers, d.Waivers) || !slices.Equal(exemptions, d.Exemptions) {
		return nil, fmt.Errorf("%w: the seeded %s does not read back as the debt set", model.ErrUsage, PolicyFile)
	}
	return data, nil
}

// sortedSet answers the values sorted and once each, empty ones dropped.
func sortedSet(values []string) []string {
	out := slices.DeleteFunc(slices.Clone(values), func(v string) bool { return v == "" })
	slices.Sort(out)
	return slices.Compact(out)
}

// waiverLines are the block entries of the waivers, one flow mapping each.
func waiverLines(waivers []policy.Waiver) string {
	lines := make([]string, 0, len(waivers))
	for _, w := range waivers {
		lines = append(lines, fmt.Sprintf("  - { id: %s, until: %s, record: %s }", quoted(w.ID), quoted(w.Until), quoted(w.Record)))
	}
	return strings.Join(lines, "\n")
}

// exceptionLines are the block entries of the exceptions.
func exceptionLines(exceptions []policy.ExceptionRule) string {
	lines := make([]string, 0, len(exceptions))
	for _, e := range exceptions {
		lines = append(lines, fmt.Sprintf("  - { check: %s, path: %s, record: %s, until: %s }",
			quoted(e.Check), quoted(e.Path), quoted(e.Record), quoted(e.Until)))
	}
	return strings.Join(lines, "\n")
}

// flowList is a flow sequence of quoted scalars.
func flowList(values []string) string {
	items := make([]string, 0, len(values))
	for _, v := range values {
		items = append(items, quoted(v))
	}
	return "[" + strings.Join(items, ", ") + "]"
}

// quoted is a double-quoted YAML scalar with every byte outside printable
// ASCII escaped; the escapes Go writes are escapes YAML reads.
func quoted(value string) string { return strconv.QuoteToASCII(value) }

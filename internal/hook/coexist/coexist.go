// Package coexist detects the Python gate of the migration (A17) on this
// machine and on a repository, and answers, per hook event and rule
// family, which of the two gates owns the blocking verdict: no file may
// receive two. The gate is known by its registration in the Claude Code
// user settings and by its registry of governed repositories; a family
// moves to the kit when the registry switches it off with its
// <family>_checks flag, which is the procedure of the install day.
package coexist

import (
	"fmt"
	"strings"
)

// Owner is who blocks on one event and family.
type Owner int

// The two owners of a verdict.
const (
	OwnerKit  Owner = iota + 1 // the kit arms the verdict
	OwnerGate                  // the Python gate keeps it until its flag is switched off
)

// String names the owner for the table.
func (o Owner) String() string {
	switch o {
	case OwnerKit:
		return "kit"
	case OwnerGate:
		return "python-gate"
	default:
		return "unknown"
	}
}

// The hook events both gates answer, in table order.
const (
	EventWrite     = "write"
	EventRead      = "read"
	EventShell     = "shell"
	EventPostWrite = "post-write"
	EventPostShell = "post-shell"
)

// The rule families the two gates share, in table order.
const (
	FamilyText       = "text"
	FamilyPaths      = "paths"
	FamilySizes      = "sizes"
	FamilySecretRead = "secret-read"
	FamilyShell      = "shell"
)

// Row is one line of the ownership table: what the kit arms on one event
// and family, and who owns it on this repository.
type Row struct {
	Event  string
	Family string
	Owner  Owner
}

// Table is the ownership table of one repository.
type Table struct {
	Gate Detection
	Rows []Row
}

// armed lists what the kit's Claude Code adapter blocks on in M1
// (specification section 8.2), in table order.
func armed() []Row {
	return []Row{
		{Event: EventWrite, Family: FamilyText},
		{Event: EventWrite, Family: FamilyPaths},
		{Event: EventWrite, Family: FamilySizes},
		{Event: EventRead, Family: FamilySecretRead},
		{Event: EventShell, Family: FamilyShell},
		{Event: EventPostWrite, Family: FamilySizes},
		{Event: EventPostShell, Family: FamilyText},
		{Event: EventPostShell, Family: FamilyPaths},
		{Event: EventPostShell, Family: FamilySizes},
	}
}

// judged maps each family to the events on which the Python gate judges
// it; the gate owns a family only while a hook of one of them is registered.
func judged() map[string][]string {
	return map[string][]string{
		FamilyText:       {EventWrite, EventPostShell},
		FamilyPaths:      {EventWrite},
		FamilySizes:      {EventPostWrite, EventPostShell},
		FamilySecretRead: {EventRead},
		FamilyShell:      {EventShell},
	}
}

// Build derives the table from a detection: a family belongs to the gate
// while the gate governs the repository, keeps a hook on an event where it
// judges the family, and the registry has not released the family.
func Build(d Detection) Table {
	rows := armed()
	for i := range rows {
		rows[i].Owner = OwnerKit
		if d.owns(rows[i].Family) {
			rows[i].Owner = OwnerGate
		}
	}
	return Table{Gate: d, Rows: rows}
}

// Owner answers who blocks on an event and family; a pair the kit does not
// arm is the kit's by default, so a caller never reads an unknown owner.
func (t Table) Owner(event, family string) Owner {
	for _, row := range t.Rows {
		if row.Event == event && row.Family == family {
			return row.Owner
		}
	}
	return OwnerKit
}

// Lines renders the table: a summary of the detection, then one row per
// event and family with its owner.
func (t Table) Lines() []string {
	lines := make([]string, 0, 2+len(t.Rows))
	lines = append(lines, t.Gate.summary(), fmt.Sprintf("%-11s %-12s %s", "event", "family", "owner"))
	for _, row := range t.Rows {
		lines = append(lines, fmt.Sprintf("%-11s %-12s %s", row.Event, row.Family, row.Owner))
	}
	return lines
}

// Refuse fails when the gate still owns a family the kit would arm, naming
// each family with the registry flag that releases it; nil means the kit
// may arm every row.
func (t Table) Refuse() error {
	var owned []string
	for _, family := range families() {
		if t.ownedByGate(family) {
			owned = append(owned, family+" (switch "+flagName(family)+" off)")
		}
	}
	if len(owned) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrOwnedElsewhere, strings.Join(owned, ", "))
}

// ownedByGate reports whether any row of the family belongs to the gate.
func (t Table) ownedByGate(family string) bool {
	for _, row := range t.Rows {
		if row.Family == family && row.Owner == OwnerGate {
			return true
		}
	}
	return false
}

// families lists the families in table order.
func families() []string {
	return []string{FamilyText, FamilyPaths, FamilySizes, FamilySecretRead, FamilyShell}
}

// flagName is the registry flag that switches a family off in the gate.
func flagName(family string) string {
	return strings.ReplaceAll(family, "-", "_") + "_checks"
}

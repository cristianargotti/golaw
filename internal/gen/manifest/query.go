package manifest

import (
	"slices"
	"strings"
)

// Lookup answers the entry of kind and name.
func (m Manifest) Lookup(kind, name string) (Entry, bool) {
	for i := range m.Entries {
		if e := &m.Entries[i]; e.Kind == kind && e.Name == name {
			return *e, true
		}
	}
	return Entry{}, false
}

// Owner answers the entry that wrote path, as regenerable or owned code.
func (m Manifest) Owner(path string) (Entry, bool) {
	for i := range m.Entries {
		if e := &m.Entries[i]; slices.Contains(e.Regenerable, path) || slices.Contains(e.Owned, path) {
			return *e, true
		}
	}
	return Entry{}, false
}

// Regenerable answers the entry that renders path again on every run; an
// owned file is never answered, because a run never rewrites it.
func (m Manifest) Regenerable(path string) (Entry, bool) {
	for i := range m.Entries {
		if e := &m.Entries[i]; slices.Contains(e.Regenerable, path) {
			return *e, true
		}
	}
	return Entry{}, false
}

// Contracting answers, in manifest order, the entries generated from the
// contract at path, so a change to it regenerates exactly those.
func (m Manifest) Contracting(path string) []Entry {
	var out []Entry
	for i := range m.Entries {
		if e := &m.Entries[i]; slices.ContainsFunc(e.Contracts, func(c Contract) bool { return c.Path == path }) {
			out = append(out, *e)
		}
	}
	return out
}

// With answers a copy holding e in place of the entry of its kind and name,
// or added, kept sorted so the CLI's writes stay deterministic; m is left as
// it was.
func (m Manifest) With(e Entry) Manifest {
	entries := slices.Clone(m.Entries)
	i := slices.IndexFunc(entries, func(old Entry) bool { return compare(old, e) == 0 })
	if i >= 0 {
		entries[i] = e
	} else {
		entries = append(entries, e)
	}
	slices.SortFunc(entries, compare)
	m.Entries = entries
	return m
}

// Area is the part of Name before the dot: the area or the package.
func (e Entry) Area() string {
	area, _, _ := strings.Cut(e.Name, ".")
	return area
}

// Type is the part of Name after the dot.
func (e Entry) Type() string {
	_, typ, _ := strings.Cut(e.Name, ".")
	return typ
}

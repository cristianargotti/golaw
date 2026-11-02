// Package manifest reads and writes the generation manifest .golaw/gen.yml:
// what golaw new and golaw gen created, so a later run renders again instead
// of trusting a hash, and a merge of two branches that each added a kind is a
// plain append (A42).
package manifest

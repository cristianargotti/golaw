// Package genguard holds the gen family of M7.9: the read only denial of
// golaw generated files and of the generation manifest, and the handwritten
// escape that takes a file out of the generator's hands. Each check judges
// only what it recognizes with certainty, so a hook denies a write only
// when the file is provably generated and names the contract to edit.
package genguard

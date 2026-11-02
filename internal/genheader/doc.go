// Package genheader owns the first line of a golaw regenerable file and the
// escape that keeps a file handwritten. It imports the standard library
// alone, so source and proof can read the header without an import cycle.
package genheader

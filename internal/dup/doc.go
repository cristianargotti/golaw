// Package dup is the duplication proxy of the golden slices (M7.10.3, A50):
// it compares Go token streams rather than text, so layout and comments
// never hide or create a copy. Its threshold sits at half the gate because
// Sonar's copy detector for Go is unproven against it (M7.15 risk 2), and
// the margin keeps a slice the proxy passes from failing the pipeline. It
// uses the standard library only, and in phase M7.1 only tests import it
// (contract 3.4 rule 4), since no command reports duplication yet.
package dup

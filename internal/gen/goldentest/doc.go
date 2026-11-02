// Package goldentest holds the golden slice harness and the template laws
// of M7.10 that the tests of every generator kind use: what Sonar's way
// gate would report on generated code, judged without Sonar ever running.
// It reads no environment and holds no package variable, so every child
// process inherits the test's own environment, and only tests import it,
// because the kit's binary never judges a template.
package goldentest

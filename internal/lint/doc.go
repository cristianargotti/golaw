// Package lint runs the pinned external linters of the non-Go surface over
// the files of the analyzer scope with the kit's configurations, parses each
// tool's machine output into issues, maps each rule to its Sonar key and
// answers one outcome per tool for the report (M6.9).
package lint

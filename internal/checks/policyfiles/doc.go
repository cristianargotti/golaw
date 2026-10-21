// Package policyfiles holds the policy family of section 6: the checks that
// judge the configuration of a repository (the lint configuration, the
// Makefile and its includes, go.mod and the decision records, the
// Dependabot, gitleaks, squawk and Redocly files, the Dockerfile, CODEOWNERS
// and the Actions files) through a declared subset of each format. Every
// check measures the tree, so it judges the worktree in a scan and the index
// in a staged scan and never runs in an agent hook. A construct outside the
// subset that a verdict depends on is a stated limit the check names in its
// reason, never a finding.
package policyfiles

// Package golaw keeps installation assets available in a single executable.
package golaw

import "embed"

// FS includes hidden metadata because plugin discovery depends on it.
//
//go:embed all:agents all:templates
var FS embed.FS

// Docs carries the rules and checklists pages, so golaw rules answers an
// agent locally instead of the agent fetching the pages from the network.
//
//go:embed docs/rules.md docs/checklists.md
var Docs embed.FS

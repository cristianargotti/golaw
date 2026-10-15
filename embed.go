// Package golaw keeps installation assets available in a single executable.
package golaw

import "embed"

// FS includes hidden metadata because plugin discovery depends on it.
//
//go:embed all:agents all:templates
var FS embed.FS

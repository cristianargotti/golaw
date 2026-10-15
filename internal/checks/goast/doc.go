// Package goast holds the Go source checks of the kit: the size laws with
// funlen semantics, the discipline rules a review would flag (environment
// reads, bare contexts, the wall clock, SQL from strings, raw error
// responses, error text as data, init-if swallows, sentinel placement,
// credential fields, goleak presence, adapter package size) and the three
// Sonar classes no linter reproduces per file. Every check reads bytes only
// through the snapshot and parses them with go/parser, so the same code
// judges a file on disk, an index blob or a candidate that does not exist
// yet.
package goast

import "github.com/cristianargotti/golaw/internal/model"

// Checks lists every check of the family in report order.
func Checks() []model.Check {
	return []model.Check{
		FileLength(),
		FunctionLength(),
		EnvRead(),
		BareContext(),
		WallClock(),
		ClockUTC(),
		SQLFromStrings(),
		RawErrorResponse(),
		ErrorTextAsData(),
		InitIfErrNil(),
		SentinelDecl(),
		SecretFields(),
	}
}

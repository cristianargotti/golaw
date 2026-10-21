package policyfiles

import (
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
)

// The linters whose settings the judgments of policy.golangci read.
const (
	linterDepguard  = "depguard"
	linterForbidigo = "forbidigo"
	linterGovet     = "govet"
)

// standardLinters are the linters golangci-lint version 2 runs under
// linters.default standard, the default.
func standardLinters() []string {
	return []string{"errcheck", linterGovet, "ineffassign", "staticcheck", "unused"}
}

// coreLinters are the linters of inventory 4.5 every Go repository runs:
// the strict set without the linters a program shape conditions.
func coreLinters() []string {
	return []string{
		"asasalint", "bidichk", "bodyclose", "canonicalheader", "containedctx", "contextcheck", "copyloopvar",
		linterDepguard, "dupl", "dupword", "durationcheck", "embeddedstructfieldcheck", "err113", "errcheck",
		"errchkjson", "errname", "errorlint", "exhaustive", "exptostd", "fatcontext", linterForbidigo,
		"forcetypeassert", "funcorder", "funlen", "gocheckcompilerdirectives", "gochecknoglobals",
		"gochecknoinits", "gochecksumtype", "gocognit", "goconst", "gocritic", "godoclint", "godot", "godox",
		"gomoddirectives", "goprintffuncname", "gosec", "gosmopolitan", linterGovet, "iface", "inamedparam",
		"ineffassign", "interfacebloat", "intrange", "maintidx", "makezero", "mirror", "misspell", "mnd",
		"modernize", "musttag", "nakedret", "nestif", "nilerr", "nilnesserr", "nilnil", "noctx", "nolintlint",
		"nonamedreturns", "nosprintfhostport", "paralleltest", "perfsprint", "prealloc", "predeclared",
		"reassign", "recvcheck", "revive", "sloglint", "staticcheck", "testableexamples", "testpackage",
		"thelper", "tparallel", "unconvert", "unparam", "unused", "usestdlibvars", "usetesting",
		"wastedassign", "whitespace", "wrapcheck",
	}
}

// shapedLinters are the linters of inventory 4.5 a capability conditions:
// the SQL linters judge a database, the wire names, metrics and spans
// linters judge a served API. testifylint judges a library no capability
// names, so the template enables it and no rule requires it.
func shapedLinters() map[string][]string {
	return map[string][]string{
		"api": {"promlinter", "spancheck", "tagliatelle"},
		"db":  {"rowserrcheck", "sqlclosecheck", "unqueryvet"},
	}
}

// requiredLinters answers the linters a repository must run under its
// capabilities, sorted.
func requiredLinters(policy *model.EffectivePolicy) []string {
	out := coreLinters()
	for capability, linters := range shapedLinters() {
		if policy.Has(capability) {
			out = append(out, linters...)
		}
	}
	slices.Sort(out)
	return out
}

// enabled answers whether a configuration runs a linter it does not
// disable, and whether that is known: under linters.default fast the set
// depends on the version of golangci-lint, so a linter the file does not
// name is unknown.
func (cfg *golangci) enabled(linter string) (bool, bool) {
	if slices.ContainsFunc(cfg.enable, func(n named) bool { return n.name == linter }) {
		return true, true
	}
	switch cfg.defaults {
	case defaultAll:
		return true, true
	case defaultFast:
		return false, false
	case defaultNone:
		return false, true
	default:
		return slices.Contains(standardLinters(), linter), true
	}
}

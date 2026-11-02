package goldentest

import (
	"fmt"
	"go/ast"
	"go/token"
	"net/netip"
	"strconv"
	"strings"
)

// The address shapes a literal field must have before it is parsed: an
// IPv4 address has three dots, an IPv6 one two colons and a decimal digit,
// so a word made of hexadecimal letters is never read as an address.
const (
	ipv4Dots    = 3
	ipv6Colons  = 2
	addressRune = "0123456789abcdefABCDEF.:"
)

// judgeRegexp is python:S5852 for every language: a template never emits
// a regular expression, so no rendering can hold one Sonar reviews.
func judgeRegexp(f goFile) []string {
	var out []string
	for _, spec := range f.file.Imports {
		if path, err := strconv.Unquote(spec.Path.Value); err == nil && path == "regexp" {
			out = append(out, at(f, spec.Pos(), lawRegexp, "imports regexp: a template never emits a regular expression"))
		}
	}
	return out
}

// judgeHTTP is typescript:S5332 for Go: a literal never holds a clear text
// URL, which the hotspot review would have to accept by hand.
func judgeHTTP(f goFile) []string {
	var out []string
	eachString(f, func(lit *ast.BasicLit, value string) {
		if strings.Contains(value, "http://") {
			out = append(out, at(f, lit.Pos(), lawHTTP, "a clear text http:// URL: use the TLS test server's https URL"))
		}
	})
	return out
}

// judgeAddresses is python:S1313 for Go: an address in a literal belongs
// to the documentation ranges, which the hotspot never reports.
func judgeAddresses(f goFile) []string {
	ranges := documentation()
	var out []string
	eachString(f, func(lit *ast.BasicLit, value string) {
		fields := strings.FieldsFunc(value, func(r rune) bool { return !strings.ContainsRune(addressRune, r) })
		for _, field := range fields {
			addr, ok := address(field)
			if !ok || documented(ranges, addr) {
				continue
			}
			detail := fmt.Sprintf("address %s outside the documentation ranges of RFC 5737 and RFC 3849", addr)
			out = append(out, at(f, lit.Pos(), lawAddresses, detail))
		}
	})
	return out
}

// address parses a field with an address shape, with or without a port.
func address(field string) (netip.Addr, bool) {
	v4 := strings.Count(field, ".") == ipv4Dots
	v6 := strings.Count(field, ":") >= ipv6Colons && strings.ContainsAny(field, "0123456789")
	if !v4 && !v6 {
		return netip.Addr{}, false
	}
	if pair, err := netip.ParseAddrPort(field); err == nil {
		return pair.Addr(), true
	}
	addr, err := netip.ParseAddr(field)
	return addr, err == nil
}

// documentation answers the ranges of RFC 5737 and RFC 3849; it is built
// on each call because the package holds no variable.
func documentation() []netip.Prefix {
	return []netip.Prefix{
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("198.51.100.0/24"),
		netip.MustParsePrefix("203.0.113.0/24"),
		netip.MustParsePrefix("2001:db8::/32"),
	}
}

func documented(ranges []netip.Prefix, addr netip.Addr) bool {
	for _, prefix := range ranges {
		if prefix.Contains(addr.Unmap()) {
			return true
		}
	}
	return false
}

// judgeCredentials is python:S2068 for Go: a credential name never takes
// a string literal, so a template's test values are made at run time.
func judgeCredentials(f goFile) []string {
	var out []string
	report := func(name *ast.Ident, value ast.Expr) {
		if isCredential(name.Name) && nonEmptyLiteral(value) {
			detail := fmt.Sprintf("a credential literal in %s: make the test value with crypto/rand at run time", name.Name)
			out = append(out, at(f, name.Pos(), lawS2068, detail))
		}
	}
	ast.Inspect(f.file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.KeyValueExpr:
			if key, ok := node.Key.(*ast.Ident); ok {
				report(key, node.Value)
			}
		case *ast.ValueSpec:
			pairNames(node.Names, node.Values, report)
		case *ast.AssignStmt:
			pairNames(node.Lhs, node.Rhs, report)
		}
		return true
	})
	return out
}

// pairNames pairs each named left side with its value when the two lists
// have one value per name.
func pairNames[T ast.Expr](names []T, values []ast.Expr, report func(*ast.Ident, ast.Expr)) {
	if len(names) != len(values) {
		return
	}
	for i, name := range names {
		if ident, ok := ast.Expr(name).(*ast.Ident); ok {
			report(ident, values[i])
		}
	}
}

// isCredential reads the name the way goast.secret-fields does: its lower
// case form, one trailing s dropped, ends in a credential word.
func isCredential(name string) bool {
	lower := strings.TrimSuffix(strings.ToLower(name), "s")
	for _, word := range []string{"apikey", "api_key", "secret", "token", "password", "passphrase", "credential"} {
		if strings.HasSuffix(lower, word) {
			return true
		}
	}
	return false
}

func nonEmptyLiteral(value ast.Expr) bool {
	lit, ok := value.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING || lit.Value[0] != interpretedQuotes {
		return false
	}
	text, err := strconv.Unquote(lit.Value)
	return err == nil && text != ""
}

// eachString calls visit with every string literal of the file and its
// unquoted value, import paths included.
func eachString(f goFile, visit func(lit *ast.BasicLit, value string)) {
	ast.Inspect(f.file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		visit(lit, value)
		return true
	})
}

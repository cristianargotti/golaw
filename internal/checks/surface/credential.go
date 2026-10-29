package surface

import (
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	detect "github.com/cristianargotti/golaw/internal/surface"
)

const (
	literalCode = "hardcoded-credential"
	literalRule = "S2068"
	// literalTail is the fix every S2068 message states (M6.6.5).
	literalTail = " (Sonar S2068): generate it at run time (secrets.token_urlsafe(16), crypto.randomUUID()) or read it from the environment."
	// pairBounds end the name and the value of a NAME=value pair inside a
	// literal: a blank, a separator of a query or a shell line, a quote.
	pairBounds = " ;&\"'`"
	// authorityEnd ends the authority of a URL inside a literal.
	authorityEnd = "/?# \"'"
)

// HardcodedCredential denies a literal credential: a string literal
// assigned, passed or bound to a name whose last word is a credential word,
// a NAME=value pair or a URL password inside a literal, and a Dockerfile
// ENV or ARG default, in tests too, because the remote analyzer reads
// tests as main code (A27).
func HardcodedCredential() model.Check {
	return check{
		spec: spec("surface.hardcoded-credential"),
		languages: []detect.Language{
			detect.JavaScript, detect.TypeScript, detect.JSX, detect.TSX, detect.HTML, detect.Python, detect.Dockerfile,
		},
		certain: true,
		judge:   judgeCredential,
	}
}

// judgeCredential routes a Python file and a Dockerfile to their readers
// and every other language to its script units.
func judgeCredential(f source, _ model.Input) []model.Finding {
	if f.info.Language == detect.Python {
		return judgePythonCredential(f)
	}
	if f.info.Language == detect.Dockerfile {
		return judgeDockerCredential(f)
	}
	var out []model.Finding
	for _, u := range scriptUnits(f) {
		out = append(out, judgeJSCredential(f, u)...)
	}
	return out
}

// nameFinding is the finding of a literal given to a credential name.
func nameFinding(f source, form, name string, line int, language detect.Language) model.Finding {
	return f.finding(literalCode, line, form+" a literal to "+name+literalTail, sonarKey(language, literalRule))
}

// literalFindings answers at most one finding for a literal no name
// consumed: a NAME=value pair whose name is a credential name, else a URL
// that holds a password in its user info.
func literalFindings(f source, value string, line int, language detect.Language) []model.Finding {
	key := sonarKey(language, literalRule)
	if name, ok := credentialPair(value); ok {
		return []model.Finding{f.finding(literalCode, line, "holds a literal value for "+name+" inside a string"+literalTail, key)}
	}
	if urlPassword(value) {
		return []model.Finding{f.finding(literalCode, line, "holds a password in the user info of a URL"+literalTail, key)}
	}
	return nil
}

// credentialPair answers the first NAME=value pair of the literal whose
// name is a credential name and whose value is not benign. A name starts
// the literal or follows a bound, so a query parameter, an environment
// line and a connection string are all read.
func credentialPair(value string) (string, bool) {
	for i := range len(value) {
		if value[i] != '=' {
			continue
		}
		name, ok := pairName(value, i)
		if !ok {
			continue
		}
		end := i + 1
		for end < len(value) && !strings.ContainsRune(pairBounds, rune(value[end])) {
			end++
		}
		if credentialName(name) && !benign(name, value[i+1:end]) {
			return name, true
		}
	}
	return "", false
}

// pairName answers the name before the equals sign at eq, when one starts
// the literal or follows a bound.
func pairName(value string, eq int) (string, bool) {
	start := eq
	for start > 0 && pairNameByte(value[start-1]) {
		start--
	}
	if start == eq || (start > 0 && !strings.ContainsRune(pairBounds, rune(value[start-1]))) {
		return "", false
	}
	return value[start:eq], true
}

// urlPassword reports a URL of the form scheme://user:pass@ whose pass is
// not benign.
func urlPassword(value string) bool {
	rest := value
	for {
		i := strings.Index(rest, "://")
		if i < 0 {
			return false
		}
		rest = rest[i+len("://"):]
		authority := rest
		if end := strings.IndexAny(rest, authorityEnd); end >= 0 {
			authority = rest[:end]
		}
		at := strings.LastIndexByte(authority, '@')
		if at < 0 {
			continue
		}
		if _, pass, ok := strings.Cut(authority[:at], ":"); ok && !benign("", pass) {
			return true
		}
	}
}

// pairNameByte reports a byte of the name of a NAME=value pair.
func pairNameByte(c byte) bool { return isLetter(c) || isDigit(c) || c == '_' }

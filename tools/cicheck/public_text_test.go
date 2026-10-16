package cicheck_test

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"unicode"
)

const privateNameDigest = "88f01e7dc6141404e93d53dcc5cd042e4801a58d6518bad55593d55ff1f25e5f"

func documentation(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".mdx", ".rst", ".adoc", ".txt":
		return true
	default:
		return false
	}
}

func newPublicTextInspector(digests []string) func(string, []byte) string {
	forbidden := make(map[string]bool, len(digests))
	for _, digest := range digests {
		forbidden[digest] = true
	}
	return func(_ string, data []byte) string {
		text := string(data)
		if strings.ContainsAny(text, "\u2013\u2014") {
			return "Documentation contains prohibited punctuation."
		}
		tokens := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
			return !unicode.IsLetter(r)
		})
		for _, token := range tokens {
			sum := sha256.Sum256([]byte(token))
			if forbidden[hex.EncodeToString(sum[:])] {
				return "Documentation names a private organization."
			}
		}
		return ""
	}
}

func inspectPublicText(path string, data []byte) string { //nolint:unused // This entry point is reserved for documentation assertions.
	return publicTextRule().inspect(path, data)
}

func publicTextRule() auditRule {
	return auditRule{
		id: "docs.public-text", rule: "rules/3@1",
		match: documentation, inspect: newPublicTextInspector([]string{privateNameDigest}),
	}
}

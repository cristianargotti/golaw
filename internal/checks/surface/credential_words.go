package surface

import (
	"slices"
	"strings"
)

// credentialWords are the default words of the remote rule S2068: a name
// whose last word is one of them names a credential.
func credentialWords() []string { return []string{"password", "passwd", "pwd", "passphrase"} }

// translatedWords are the labels a localized view carries for the
// credential word, which read as text and never as a credential.
func translatedWords() []string {
	return []string{"senha", "contraseña", "contrasena", "clave", "passwort", "wachtwoord", "palavra-passe"}
}

// placeholderMarks are the marks of a value a template, a printf or the
// environment fills later.
func placeholderMarks() []string { return []string{"${", "{{", "%s", "%("} }

// credentialName reports whether the last word of the name is a credential
// word.
func credentialName(name string) bool {
	words := nameWords(name)
	return len(words) > 0 && slices.Contains(credentialWords(), strings.ToLower(words[len(words)-1]))
}

// nameWords splits a name on _, - and . and on its case changes: a lower
// case byte before an upper case one, and the last upper case byte of a
// run before a lower case one, so DBPassword is DB and Password.
func nameWords(name string) []string {
	var words []string
	start := 0
	flush := func(end int) {
		if end > start {
			words = append(words, name[start:end])
		}
	}
	for i := range len(name) {
		c := name[i]
		if c == '_' || c == '-' || c == '.' {
			flush(i)
			start = i + 1
			continue
		}
		next := i+1 < len(name) && lowerOrDigit(name[i+1])
		if i > start && isUpper(c) && (lowerOrDigit(name[i-1]) || (isUpper(name[i-1]) && next)) {
			flush(i)
			start = i
		}
	}
	flush(len(name))
	return words
}

// benign reports a value that cannot be a credential, so a certain denial
// never fires on it: empty, the name or its label, stars or x marks, a
// sentence or a label with a blank inside, a translated credential word,
// or a placeholder filled later.
func benign(name, value string) bool {
	label := strings.TrimSuffix(value, ":")
	switch {
	case value == "", placeholder(value), onlyOf(value, "*"), onlyOf(value, "xX"):
		return true
	case strings.ContainsAny(strings.TrimSpace(value), " \t"):
		return true
	case slices.ContainsFunc(translatedWords(), func(w string) bool { return strings.EqualFold(label, w) }):
		return true
	}
	words := nameWords(name)
	if len(words) == 0 {
		return false
	}
	return strings.EqualFold(label, name) || strings.EqualFold(label, words[len(words)-1]) ||
		strings.EqualFold(label, strings.Join(words, " "))
}

// placeholder reports a value a template, a printf or the environment
// fills, or one spelled as a placeholder between angle brackets.
func placeholder(value string) bool {
	if strings.HasPrefix(value, "$") || (strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">")) {
		return true
	}
	return slices.ContainsFunc(placeholderMarks(), func(m string) bool { return strings.Contains(value, m) })
}

// onlyOf reports a value made only of the given bytes.
func onlyOf(value, set string) bool {
	return strings.Trim(value, set) == ""
}

func isUpper(c byte) bool { return c >= 'A' && c <= 'Z' }

func lowerOrDigit(c byte) bool { return (c >= 'a' && c <= 'z') || isDigit(c) }

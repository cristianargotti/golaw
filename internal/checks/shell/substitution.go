package shell

// substitutions answers the scripts of the command substitutions the
// scanner keeps inside a word: a $( ) inside double quotes, which a POSIX
// shell and PowerShell both run, and in a POSIX shell the backquotes,
// outside quotes or inside double ones. A substitution outside quotes is
// split by its parentheses already. The escape of the dialect makes the
// next rune text, and single quotes hold no substitution.
func substitutions(command string, d dialect) []string {
	var out []string
	runes := []rune(command)
	quote := rune(0)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case quote == '\'':
			if r == '\'' {
				quote = 0
			}
		case r == rune(d):
			i++
		case r == '\'' && quote == 0:
			quote = r
		case r == '"' && quote == 0:
			quote = r
		case r == '"':
			quote = 0
		case r == '`' && d == posix:
			end := closing(runes, i+1, '`', rune(d))
			out = append(out, string(runes[i+1:end]))
			i = end
		case r == '$' && quote == '"' && i+1 < len(runes) && runes[i+1] == '(':
			end := closing(runes, i+2, ')', rune(d))
			out = append(out, string(runes[i+2:end]))
			i = end
		}
	}
	return out
}

// closing answers the index of the rune that closes a substitution opened
// before from, or the end of the command when none does: a backquote
// closes at the next one the escape of the dialect does not escape, and a
// parenthesis at the one that balances it, past the quoted runes.
func closing(runes []rune, from int, end, escape rune) int {
	depth, quote := 1, rune(0)
	for i := from; i < len(runes); i++ {
		switch r := runes[i]; {
		case r == escape:
			i++
		case end == '`' && r == end:
			return i
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == '(':
			depth++
		case r == ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(runes)
}

package surface

import "slices"

// vmSources are the names that load the vm module of node.
func vmSources() []string { return []string{"vm", "node:vm"} }

// vmCalls and vmClasses are the members of the module that compile text,
// the list the code-eval rule of the sonarjs set reads.
func vmCalls() []string {
	return []string{"runInThisContext", "runInNewContext", "runInContext", "compileFunction"}
}

func vmClasses() []string { return []string{"Script", "SourceTextModule"} }

const (
	vmPrefix = "vm."
	// requireTokens is the length of require ( "vm" ), after which a
	// member of an inline require follows.
	requireTokens = 4
)

// vmScope is what a unit binds to the vm module: the local names of the
// module itself and the local names of its members.
type vmScope struct {
	modules map[string]bool
	locals  map[string]string
}

// vmBindings reads the require calls and the imports of the vm module in
// a unit; only the two module names bind, so a local helper named vm is
// never the module.
func vmBindings(tokens []token) vmScope {
	s := vmScope{modules: map[string]bool{}, locals: map[string]string{}}
	for i := range tokens {
		switch {
		case requiresVM(tokens, i) && isPunctAt(tokens, i-1, "=") && !isPunctAt(tokens, i+requireTokens, "."):
			s.bindRequire(tokens, i-2)
		case isIdentAt(tokens, i, "import"):
			s.bindImport(tokens, i)
		}
	}
	return s
}

// requiresVM reports a require call of the vm module at i.
func requiresVM(tokens []token, i int) bool {
	if !isIdentAt(tokens, i, "require") || member(tokens, i) || !isPunctAt(tokens, i+1, "(") {
		return false
	}
	name := i + 2
	return name < len(tokens) && tokens[name].kind == tokString &&
		slices.Contains(vmSources(), stringValue(tokens[name])) && isPunctAt(tokens, name+1, ")")
}

// bindRequire binds what the left side ending at last names: a declared
// identifier is the module, a destructuring pattern names its members.
func (s vmScope) bindRequire(tokens []token, last int) {
	if last < 0 {
		return
	}
	if tokens[last].kind == tokIdent && last > 0 && slices.Contains([]string{"const", "let", "var"}, tokens[last-1].text) {
		s.modules[tokens[last].text] = true
		return
	}
	if !isPunctAt(tokens, last, "}") {
		return
	}
	depth := 0
	for open := last; open >= 0; open-- {
		switch {
		case isPunctAt(tokens, open, "}"):
			depth++
		case isPunctAt(tokens, open, "{"):
			depth--
		}
		if depth == 0 {
			s.bindMembers(tokens[open+1:last], ":")
			return
		}
	}
}

// bindMembers binds the entries of a destructuring pattern or a named
// import list: a name alone, or a member and its local name after the
// separator (a colon or as).
func (s vmScope) bindMembers(entries []token, separator string) {
	for entry := range splitTokens(entries) {
		if len(entry) == 0 || entry[0].kind != tokIdent {
			continue
		}
		name, local := entry[0].text, entry[0].text
		if len(entry) >= 3 && entry[1].text == separator && entry[2].kind == tokIdent {
			local = entry[2].text
		}
		if slices.Contains(vmCalls(), name) || slices.Contains(vmClasses(), name) {
			s.locals[local] = name
		}
	}
}

// splitTokens yields the entries of a list split at its top level commas.
func splitTokens(tokens []token) func(func([]token) bool) {
	return func(yield func([]token) bool) {
		start, depth := 0, 0
		for i, t := range tokens {
			switch {
			case t.kind == tokPunct && closer(t.text) != "":
				depth++
			case t.kind == tokPunct && (t.text == ")" || t.text == "]" || t.text == "}"):
				depth--
			case isPunctAt(tokens, i, ",") && depth == 0:
				if !yield(tokens[start:i]) {
					return
				}
				start = i + 1
			}
		}
		yield(tokens[start:])
	}
}

// bindImport binds the clause of an import of the vm module at i: a
// default or namespace name is the module, a named list names members.
func (s vmScope) bindImport(tokens []token, i int) {
	from := i + 1
	for from < len(tokens) && !isIdentAt(tokens, from, "from") && !isPunctAt(tokens, from, ";") {
		from++
	}
	if !isIdentAt(tokens, from, "from") || from+1 >= len(tokens) || tokens[from+1].kind != tokString ||
		!slices.Contains(vmSources(), stringValue(tokens[from+1])) {
		return
	}
	for k := i + 1; k < from; k++ {
		switch {
		case isPunctAt(tokens, k, "*") && isIdentAt(tokens, k+1, "as") && k+2 < from:
			s.modules[tokens[k+2].text] = true
			k += 2
		case isPunctAt(tokens, k, "{"):
			end := min(closing(tokens, k), from)
			s.bindMembers(tokens[k+1:end], "as")
			k = end
		case tokens[k].kind == tokIdent:
			s.modules[tokens[k].text] = true
		}
	}
}

// use reads a call of the vm module at i: a member of a module binding, a
// class of it constructed, a bound member called, or the module required
// inline.
func (s vmScope) use(tokens []token, i int) (string, int, bool) {
	t := tokens[i]
	switch {
	case t.kind == tokIdent && s.modules[t.text] && !member(tokens, i) && memberCall(tokens, i+1, vmCalls()):
		return vmPrefix + tokens[i+2].text, i, true
	case isIdentAt(tokens, i, keywordNew) && i+1 < len(tokens) && s.modules[tokens[i+1].text] && memberCall(tokens, i+2, vmClasses()):
		return keywordNew + " " + vmPrefix + tokens[i+3].text, i, true
	case requiresVM(tokens, i) && memberCall(tokens, i+requireTokens, vmCalls()):
		return vmPrefix + tokens[i+requireTokens+1].text, i, true
	case t.kind == tokIdent && s.locals[t.text] != "" && !member(tokens, i) &&
		!isIdentAt(tokens, i-1, "function") && isPunctAt(tokens, i+1, "("):
		name := s.locals[t.text]
		constructed := isIdentAt(tokens, i-1, keywordNew)
		if slices.Contains(vmClasses(), name) {
			return keywordNew + " " + vmPrefix + name, i - 1, constructed
		}
		return vmPrefix + name, i, !constructed
	}
	return "", 0, false
}

// memberCall reports a dot at i, a member among names and a call.
func memberCall(tokens []token, i int, names []string) bool {
	return isPunctAt(tokens, i, ".") && i+2 < len(tokens) && tokens[i+1].kind == tokIdent &&
		slices.Contains(names, tokens[i+1].text) && isPunctAt(tokens, i+2, "(")
}

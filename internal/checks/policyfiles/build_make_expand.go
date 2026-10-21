package policyfiles

import (
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

type makeExpansion struct {
	reader    *makeReader
	remaining int
}

func makeLiteralName(name string) bool {
	if name == "" {
		return false
	}
	for _, char := range name {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') && !strings.ContainsRune("_.-", char) {
			return false
		}
	}
	return true
}

func makeReferences(text string) ([]string, bool) {
	var names []string
	for i := 0; i < len(text); i++ {
		if text[i] != '$' {
			continue
		}
		if !makeReferenceStart(text, i) {
			return names, false
		}
		end := makeReferenceEnd(text, i)
		if end < i+2 || text[end] != makeCloser(text[i+1]) {
			return names, false
		}
		name := text[i+2 : end]
		if !makeLiteralName(name) {
			return names, false
		}
		names = append(names, name)
		i = end
	}
	return names, true
}

func makeCloser(opener byte) byte {
	if opener == '(' {
		return ')'
	}
	return '}'
}

func (r *makeReader) markTainted() {
	declared := make(map[string]bool)
	for _, event := range r.events {
		if !event.variable {
			continue
		}
		v := r.result.vars[event.index]
		if v.target == "" && event.op != makeExport && event.op != makeUnexport {
			declared[v.name] = true
		}
		_, literal := makeReferences(v.value)
		if v.target == "" && (v.condition != "" || event.op == makeDefine || event.op == "!=" || !literal) {
			r.tainted[v.name] = true
		}
	}
	for r.spreadTaint(declared) {
	}
}

func (r *makeReader) spreadTaint(declared map[string]bool) bool {
	changed := false
	for _, v := range r.result.vars {
		if v.target != "" || r.tainted[v.name] {
			continue
		}
		names, _ := makeReferences(v.value)
		for _, name := range names {
			if !declared[name] || r.tainted[name] {
				r.tainted[v.name], changed = true, true
				break
			}
		}
	}
	return changed
}

func (r *makeReader) expandWords(words []string) []string {
	var expanded []string
	for _, word := range words {
		value, ok := r.expand(word)
		if !ok {
			expanded = append(expanded, word)
			continue
		}
		expanded = append(expanded, makeWords(value)...)
	}
	return expanded
}

func (r *makeReader) expand(text string) (string, bool) {
	expansion := makeExpansion{reader: r, remaining: model.MaxReadBytes}
	return expansion.expand(text, nil)
}

func (e *makeExpansion) expand(text string, stack []string) (string, bool) {
	if !e.spend(len(text)) {
		return text, false
	}
	if _, literal := makeReferences(text); !literal {
		return text, false
	}
	expanded := make([]byte, 0, len(text))
	for i := 0; i < len(text); i++ {
		if text[i] != '$' {
			expanded = append(expanded, text[i])
			continue
		}
		end := makeReferenceEnd(text, i)
		value, ok := e.resolve(text[i+2:end], stack)
		if !ok {
			return text, false
		}
		expanded = append(expanded, value...)
		i = end
	}
	return string(expanded), true
}

func (e *makeExpansion) resolve(name string, stack []string) (string, bool) {
	const maxReferences = 32
	binding, found := e.reader.bindings[name]
	if !found || !binding.set || e.reader.tainted[name] ||
		len(stack) >= maxReferences || slices.Contains(stack, name) {
		return "", false
	}
	if binding.immediate {
		return binding.value, binding.literal && e.spend(len(binding.value))
	}
	return e.expand(binding.value, append(stack, name))
}

func (e *makeExpansion) spend(size int) bool {
	if size > e.remaining {
		return false
	}
	e.remaining -= size
	return true
}

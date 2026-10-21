package policyfiles

import "strings"

const (
	makeExport   = "export"
	makeUnexport = "unexport"
)

type makeBinding struct {
	value     string
	raw       string
	exported  bool
	immediate bool
	literal   bool
	set       bool
}

func (r *makeReader) variable(line makeLine, condition string) bool {
	text := line.text
	targets := []string{""}
	colon, equal := makeDelimiter(text, ":"), makeDelimiter(text, "=")
	if colon >= 0 && (equal < 0 || colon < makeAssignmentStart(text, equal)) {
		if equal < 0 || strings.HasPrefix(text[colon:], "::") {
			return false
		}
		targets, text = makeWords(text[:colon]), strings.TrimSpace(text[colon+1:])
	}
	export := ""
	words := makeWords(text)
	if len(words) != 0 && (words[0] == makeExport || words[0] == makeUnexport) {
		export, text = words[0], strings.TrimSpace(strings.TrimPrefix(text, words[0]))
	}
	name, op, value := makeAssignment(text)
	names := []string{name}
	if op == "" {
		if export == "" {
			return false
		}
		names, op = makeWords(text), export
	}
	for _, target := range targets {
		for _, name := range names {
			r.recordVar(makeVar{
				name: name, value: value, exported: export == makeExport,
				target: target, file: line.file, line: line.line, condition: condition,
			}, op)
		}
	}
	return true
}

func makeAssignment(text string) (string, string, string) {
	equal := makeDelimiter(text, "=")
	if equal < 0 {
		return "", "", ""
	}
	start := makeAssignmentStart(text, equal)
	name := strings.TrimSpace(text[:start])
	if !makeLiteralName(name) {
		return "", "", ""
	}
	return name, text[start : equal+1], strings.TrimSpace(text[equal+1:])
}

func makeAssignmentStart(text string, equal int) int {
	for _, op := range []string{":::=", "::=", ":=", "?=", "+=", "!="} {
		start := equal + 1 - len(op)
		if start >= 0 && text[start:equal+1] == op {
			return start
		}
	}
	return equal
}

func (r *makeReader) recordVar(v makeVar, op string) {
	r.events = append(r.events, makeEvent{index: len(r.result.vars), op: op, variable: true})
	r.result.vars = append(r.result.vars, v)
}

func (r *makeReader) assign(event makeEvent) {
	v := &r.result.vars[event.index]
	key := v.name
	if v.target != "" || v.condition != "" {
		key = v.condition + "\x00" + v.target + "\x00" + v.name
	}
	binding := r.bindings[key]
	switch event.op {
	case makeExport:
		binding.exported = true
	case makeUnexport:
		binding.exported = false
	default:
		binding = r.assigned(binding, v.value, event.op)
		binding.exported = binding.exported || v.exported
	}
	v.value, v.exported = binding.raw, binding.exported
	r.bindings[key] = binding
}

func (r *makeReader) assigned(binding makeBinding, value, op string) makeBinding {
	if op == "?=" && binding.set {
		return binding
	}
	if op == "+=" && binding.set {
		binding.raw = strings.TrimSpace(binding.raw + " " + value)
		literal := true
		if binding.immediate {
			value, literal = r.expand(value)
		}
		binding.value = strings.TrimSpace(binding.value + " " + value)
		binding.literal = binding.literal && literal
		return binding
	}
	binding.value, binding.raw, binding.set, binding.literal = value, value, true, true
	binding.immediate = op == ":=" || op == "::=" || op == ":::="
	if binding.immediate {
		binding.value, binding.literal = r.expand(value)
	}
	return binding
}

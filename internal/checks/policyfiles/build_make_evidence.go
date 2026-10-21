package policyfiles

import (
	"path"
	"strings"
	"unicode/utf8"
)

// makeTargetSpan is a literal piece of a raw target token and the fewest
// bytes the unknown span before it must supply: one for a pattern stem,
// none for an unresolved reference.
type makeTargetSpan struct {
	text    string
	minimum int
}

// makeTargetLimit defers only evidence that could supply an absent target,
// because an unrelated expression cannot excuse a definite missing target.
func makeTargetLimit(m makefile, target string) error {
	if _, found := m.rule(target); found {
		return nil
	}
	for _, rule := range m.rules {
		for _, token := range rule.targets {
			if makeUnknownTarget(token, target) {
				return outside(rule.file, rule.line, token)
			}
		}
	}
	if rule, found := m.conditional(target); found {
		return outside(rule.file, rule.line,
			target+" defined only inside the conditional block opened at "+rule.condition)
	}
	for _, include := range m.missing {
		if include.condition != "" {
			return outside(include.file, include.line,
				"conditional include "+include.name+" the snapshot does not hold may define "+target)
		}
	}
	if len(m.opaque) > 0 {
		line := m.opaque[0]
		return outside(line.file, line.line, line.text+" at the top level may define "+target)
	}
	return nil
}

// makeUnknownTarget answers whether a raw token could name the target once
// make expanded it, keeping its literal prefix, pieces and suffix.
func makeUnknownTarget(token, target string) bool {
	spans := makeTargetSpans(token)
	if makePatternBasename(spans) {
		target = path.Base(target)
	}
	if len(spans) < 2 || !strings.HasPrefix(target, spans[0].text) {
		return false
	}
	target = strings.TrimPrefix(target, spans[0].text)
	for _, span := range spans[1 : len(spans)-1] {
		if len(target) < span.minimum {
			return false
		}
		target = target[span.minimum:]
		index := strings.Index(target, span.text)
		if index < 0 {
			return false
		}
		target = target[index+len(span.text):]
	}
	last := spans[len(spans)-1]
	return len(target) >= last.minimum+len(last.text) && strings.HasSuffix(target, last.text)
}

// Patterns without a directory match basenames when make remakes an include.
func makePatternBasename(spans []makeTargetSpan) bool {
	pattern := false
	for _, span := range spans {
		if strings.Contains(span.text, "/") {
			return false
		}
		pattern = pattern || span.minimum != 0
	}
	return pattern
}

// makeTargetSpans splits a raw target token at its first pattern stem and
// its references; an escaped dollar stays literal text.
func makeTargetSpans(token string) []makeTargetSpan {
	var spans []makeTargetSpan
	start, index, minimum := 0, 0, 0
	pattern := false
	for index < len(token) {
		end, nextMinimum := index, 0
		switch {
		case token[index] == '%' && !pattern:
			pattern, end, nextMinimum = true, index+1, 1
		case token[index] == '$':
			if index+1 < len(token) && token[index+1] == '$' {
				index += 2
				continue
			}
			end = makeTargetReferenceEnd(token, index)
		}
		if end == index {
			index++
			continue
		}
		spans = append(spans, makeTargetSpan{text: token[start:index], minimum: minimum})
		start, index, minimum = end, end, nextMinimum
	}
	return append(spans, makeTargetSpan{text: token[start:], minimum: minimum})
}

// makeTargetReferenceEnd answers the end of a reference: a balanced one in
// parentheses or braces, or the single character a short one names.
func makeTargetReferenceEnd(token string, index int) int {
	if makeReferenceStart(token, index) {
		return makeReferenceEnd(token, index) + 1
	}
	_, size := utf8.DecodeRuneInString(token[index+1:])
	return index + 1 + size
}

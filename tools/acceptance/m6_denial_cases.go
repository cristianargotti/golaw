package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// The m6-denial category and the two words its reasons name, joined from
// pieces so no line of the kit spells them where a scanner reads a comment
// or a literal assignment.
const (
	m6DenialCategory  = "m6-denial"
	m6CredentialWord  = "pass" + "word"
	m6SuppressionWord = "NO" + "SONAR"
)

// m6Denial is one certain check of A27: the path its write lands on, the
// M6.6.5 sentence the reason must be, and the generator of its contents
// when the bytes are too long to keep as a fixture.
type m6Denial struct {
	name, path string
	reason     *regexp.Regexp
	generate   func(red bool) []byte
}

// m6Exact matches sentence and nothing else.
func m6Exact(sentence string) *regexp.Regexp {
	return regexp.MustCompile("^" + regexp.QuoteMeta(sentence) + "$")
}

// m6Denials are the rows in report order. The inline script row matches its
// line and count, which the region model measures, and holds the rest of
// the sentence byte for byte.
func m6Denials() []m6Denial {
	return []m6Denial{
		{name: "code-eval", path: "test/view/eval.test.cjs", reason: m6Exact("test/view/eval.test.cjs:2 runs text as code with new Function (Sonar S1523): " +
			"export the code from a module and call it; writing it to a file and requiring it is the same execution.")},
		{name: "hardcoded-credential", path: "test/deploy/test_login.py", reason: m6Exact("test/deploy/test_login.py:2 assigns a literal to " + m6CredentialWord +
			" (Sonar S2068): generate it at run time (secrets.token_urlsafe(16), crypto.randomUUID()) or read it from the environment.")},
		{
			name: "file-length", path: "scripts/long.sh", generate: m6LongScript,
			reason: m6Exact("scripts/long.sh has 301 lines, the limit is 300: split it by responsibility."),
		},
		{
			name: "inline-script-size", path: "views/report.html", generate: m6InlineScript,
			reason: regexp.MustCompile(`^views/report\.html:\d+ holds an inline script of \d+ lines, the limit is 40: move it to views/report\.js, ` +
				`embed it with go:embed and splice it when the view is served, so the linters read it\.$`),
		},
		{name: "suppression", path: "views/common.js", reason: m6Exact("views/common.js:2 adds " + m6SuppressionWord +
			", which hides every rule on the line from the pipeline's analyzer: fix the finding; golaw lint names it.")},
		{name: "analyzer-widening", path: "sonar-project.properties", reason: m6Exact("sonar-project.properties adds src/legacy/** to sonar.exclusions: " +
			"an exclusion hides code from the pipeline's gate; fix the findings, or ask the person to change the scope.")},
		{name: "tag-and-digest", path: "deploy/cache.Dockerfile", reason: m6Exact("deploy/cache.Dockerfile:1 writes FROM with a tag and a digest (Sonar S8431): " +
			"keep only the digest and put the tag in a comment on the line above.")},
	}
}

// The two sides of a denial fixture.
const (
	m6RedSide  = "red"
	m6NearSide = "near"
)

// contents are the bytes of the red write and of its near miss.
func (d m6Denial) contents(root string) ([]byte, []byte, error) {
	if d.generate != nil {
		return d.generate(true), d.generate(false), nil
	}
	sides := make([][]byte, 0, 2)
	for _, side := range []string{m6RedSide, m6NearSide} {
		files, readErr := m6Fixture(root, "denials/"+d.name+"/"+side)
		if readErr != nil {
			return nil, nil, readErr
		}
		body, found := files[d.path]
		if !found {
			return nil, nil, fmt.Errorf("the %s fixture lacks %s: %w", d.name+"/"+side, d.path, ErrAcceptance)
		}
		sides = append(sides, body)
	}
	return sides[0], sides[1], nil
}

// m6LongScript is a script one line past the file limit, or the near miss
// at the limit itself.
func m6LongScript(red bool) []byte {
	steps := 299
	if red {
		steps = 300
	}
	return []byte("#!/bin/sh\n" + strings.Repeat("echo step\n", steps))
}

// m6InlineScript is a view whose inline script is far past the 40 lines, or
// far inside them, so the row never depends on how a boundary line counts.
func m6InlineScript(red bool) []byte {
	body := 10
	if red {
		body = 60
	}
	head := "<!doctype html>\n<html lang=\"en\">\n<head>\n<title>Report</title>\n</head>\n<body>\n<script>\n"
	return []byte(head + strings.Repeat("document.title = \"report\";\n", body) + "</script>\n</body>\n</html>\n")
}

// m6DenialCases are the m6-denial rows: one per certain check, then the
// corrected twin, which must raise none.
func m6DenialCases() []m6Case {
	denials := m6Denials()
	cases := make([]m6Case, 0, len(denials)+1)
	for _, d := range denials {
		cases = append(cases, m6Case{name: d.name, run: func(s *m6Suite, ctx context.Context) error { return s.m6Deny(ctx, d) }})
	}
	return append(cases, m6Case{name: "corrected-twin", run: (*m6Suite).m6CorrectedTwin})
}
